#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd"
    exit 1
  fi
}

require_cmd curl
require_cmd jq
require_cmd python3
require_cmd docker

BASE_URL="${BASE_URL:-http://localhost:8080}"
GRAPHQL_URL="${GRAPHQL_URL:-${BASE_URL}/query}"
METRICS_URL="${METRICS_URL:-${BASE_URL}/metrics}"
REQUESTS="${REQUESTS:-120}"
REPORT_DIR="${REPORT_DIR:-loadtest}"
REPORT_JSON="${REPORT_JSON:-${REPORT_DIR}/dataloader-impact-report.json}"
REPORT_MD="${REPORT_MD:-${REPORT_DIR}/dataloader-impact-resume.md}"

mkdir -p "$REPORT_DIR"

graphql_payload='{"query":"query { products(pagination:{limit:20, offset:0}, sorting:{field:PRICE, order:ASC}) { id name category { id name } } }"}'

wait_for_health() {
  for _ in $(seq 1 60); do
    if curl -sS -m 2 "$BASE_URL/healthz" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "API did not become healthy in time"
  exit 1
}

snapshot_metrics() {
  local file="$1"
  curl -sS "$METRICS_URL" > "$file"
}

metric_value() {
  local file="$1"
  local key="$2"
  awk -v k="$key" '$1==k {print $2; found=1} END{if(!found) print 0}' "$file"
}

delta() {
  python3 - <<PY
b=float("$1")
a=float("$2")
print(a-b)
PY
}

run_phase() {
  local dataloader_enabled="$1"
  local phase_name="$2"
  local output_prefix="$3"

  echo "Starting phase: $phase_name (DATALOADER_ENABLED=$dataloader_enabled)"
  DATALOADER_ENABLED="$dataloader_enabled" docker compose up -d --build app >/dev/null
  wait_for_health

  local metrics_before
  local metrics_after
  metrics_before="$(mktemp)"
  metrics_after="$(mktemp)"

  snapshot_metrics "$metrics_before"

  local success=0
  local errors=0
  local sql_total=0
  local latencies_file
  latencies_file="$(mktemp)"

  local start_ts end_ts
  start_ts="$(python3 - <<'PY'
import time
print(time.time())
PY
)"

  for _ in $(seq 1 "$REQUESTS"); do
    local headers_file body_file
    headers_file="$(mktemp)"
    body_file="$(mktemp)"

    local time_total
    time_total="$(curl -sS -D "$headers_file" -o "$body_file" \
      -w '%{time_total}' \
      -H 'Content-Type: application/json' \
      -d "$graphql_payload" \
      "$GRAPHQL_URL" || echo 0)"

    local sql_count
    sql_count="$(awk 'tolower($1)=="x-sql-statements:" {print $2}' "$headers_file" | tr -d '\r')"
    sql_count="${sql_count:-0}"

    local gql_errors
    gql_errors="$(jq '.errors | length // 0' "$body_file" 2>/dev/null || echo 1)"

    if [[ "$gql_errors" == "0" ]]; then
      success=$((success + 1))
    else
      errors=$((errors + 1))
    fi

    sql_total="$(python3 - <<PY
print(float("$sql_total") + float("$sql_count"))
PY
)"

    echo "$time_total" >> "$latencies_file"

    rm -f "$headers_file" "$body_file"
  done

  end_ts="$(python3 - <<'PY'
import time
print(time.time())
PY
)"

  snapshot_metrics "$metrics_after"

  local elapsed throughput avg_latency_ms p95_ms p99_ms error_rate_pct avg_sql_per_req
  elapsed="$(python3 - <<PY
print(float("$end_ts") - float("$start_ts"))
PY
)"
  throughput="$(python3 - <<PY
elapsed=float("$elapsed")
req=float("$REQUESTS")
print(0 if elapsed == 0 else req/elapsed)
PY
)"
  avg_latency_ms="$(python3 - <<PY
vals=[float(x.strip()) for x in open("$latencies_file") if x.strip()]
print((sum(vals)/len(vals))*1000 if vals else 0)
PY
)"
  p95_ms="$(python3 - <<PY
vals=sorted([float(x.strip()) for x in open("$latencies_file") if x.strip()])
if not vals:
    print(0)
else:
    idx=max(0, min(len(vals)-1, int(round((len(vals)-1)*0.95))))
    print(vals[idx]*1000)
PY
)"
  p99_ms="$(python3 - <<PY
vals=sorted([float(x.strip()) for x in open("$latencies_file") if x.strip()])
if not vals:
    print(0)
else:
    idx=max(0, min(len(vals)-1, int(round((len(vals)-1)*0.99))))
    print(vals[idx]*1000)
PY
)"
  error_rate_pct="$(python3 - <<PY
err=float("$errors")
req=float("$REQUESTS")
print(0 if req == 0 else (err/req)*100)
PY
)"
  avg_sql_per_req="$(python3 - <<PY
sql_total=float("$sql_total")
req=float("$REQUESTS")
print(0 if req == 0 else sql_total/req)
PY
)"

  local db_before db_after db_delta
  db_before="$(metric_value "$metrics_before" 'db_query_duration_seconds_count{repository="category_repository",method="GetByID"}')"
  db_after="$(metric_value "$metrics_after" 'db_query_duration_seconds_count{repository="category_repository",method="GetByID"}')"
  db_delta="$(delta "$db_before" "$db_after")"

  local resolver_before_sum resolver_after_sum resolver_before_count resolver_after_count resolver_avg_ms
  resolver_before_sum="$(metric_value "$metrics_before" 'graphql_resolver_duration_seconds_sum{operation="productCategory"}')"
  resolver_after_sum="$(metric_value "$metrics_after" 'graphql_resolver_duration_seconds_sum{operation="productCategory"}')"
  resolver_before_count="$(metric_value "$metrics_before" 'graphql_resolver_duration_seconds_count{operation="productCategory"}')"
  resolver_after_count="$(metric_value "$metrics_after" 'graphql_resolver_duration_seconds_count{operation="productCategory"}')"

  resolver_avg_ms="$(python3 - <<PY
sum_delta=float("$resolver_after_sum")-float("$resolver_before_sum")
count_delta=float("$resolver_after_count")-float("$resolver_before_count")
print(0 if count_delta == 0 else (sum_delta/count_delta)*1000)
PY
)"

  cat > "${REPORT_DIR}/${output_prefix}.json" <<JSON
{
  "phase": "$phase_name",
  "dataloader_enabled": $dataloader_enabled,
  "requests": $REQUESTS,
  "success": $success,
  "errors": $errors,
  "error_rate_pct": $error_rate_pct,
  "throughput_rps": $throughput,
  "avg_latency_ms": $avg_latency_ms,
  "p95_latency_ms": $p95_ms,
  "p99_latency_ms": $p99_ms,
  "avg_sql_queries_per_request": $avg_sql_per_req,
  "category_getbyid_db_calls": $db_delta,
  "productCategory_resolver_avg_ms": $resolver_avg_ms
}
JSON

  rm -f "$latencies_file" "$metrics_before" "$metrics_after"
}

run_phase false "before_dataloader" "dataloader-before"
run_phase true "after_dataloader" "dataloader-after"

python3 - <<PY
import json
from pathlib import Path

report_dir = Path("$REPORT_DIR")
before = json.loads((report_dir / "dataloader-before.json").read_text())
after = json.loads((report_dir / "dataloader-after.json").read_text())

def pct_reduce(before_v, after_v):
    if before_v == 0:
        return 0.0
    return ((before_v - after_v) / before_v) * 100

def pct_improve(before_v, after_v):
    if before_v == 0:
        return 0.0
    return ((after_v - before_v) / before_v) * 100

summary = {
    "before": before,
    "after": after,
    "improvement": {
        "sql_queries_reduction_pct": pct_reduce(before["avg_sql_queries_per_request"], after["avg_sql_queries_per_request"]),
        "resolver_latency_reduction_pct": pct_reduce(before["productCategory_resolver_avg_ms"], after["productCategory_resolver_avg_ms"]),
        "p95_latency_reduction_pct": pct_reduce(before["p95_latency_ms"], after["p95_latency_ms"]),
        "throughput_improvement_pct": pct_improve(before["throughput_rps"], after["throughput_rps"]),
        "db_calls_reduction_pct": pct_reduce(before["category_getbyid_db_calls"], after["category_getbyid_db_calls"])
    },
    "formulas": {
        "db_calls_reduction_pct": "(before_db_calls - after_db_calls) / before_db_calls * 100",
        "resolver_latency_reduction_pct": "(before_resolver_ms - after_resolver_ms) / before_resolver_ms * 100",
        "throughput_improvement_pct": "(after_rps - before_rps) / before_rps * 100"
    }
}

Path("$REPORT_JSON").write_text(json.dumps(summary, indent=2))

resume_md = Path("$REPORT_MD")
resume_md.write_text(
    "# Dataloader Impact Benchmark\\n\\n"
    + f"- Avg SQL queries/request: {before['avg_sql_queries_per_request']:.2f} -> {after['avg_sql_queries_per_request']:.2f} "
      f"({summary['improvement']['sql_queries_reduction_pct']:.2f}% reduction)\\n"
    + f"- productCategory resolver avg latency: {before['productCategory_resolver_avg_ms']:.3f} ms -> {after['productCategory_resolver_avg_ms']:.3f} ms "
      f"({summary['improvement']['resolver_latency_reduction_pct']:.2f}% reduction)\\n"
    + f"- Throughput: {before['throughput_rps']:.2f} rps -> {after['throughput_rps']:.2f} rps "
      f"({summary['improvement']['throughput_improvement_pct']:.2f}% improvement)\\n"
    + f"- Category GetByID DB calls: {before['category_getbyid_db_calls']:.0f} -> {after['category_getbyid_db_calls']:.0f} "
      f"({summary['improvement']['db_calls_reduction_pct']:.2f}% reduction)\\n\\n"
    + "## Resume-ready bullets\\n\\n"
    + f"- Reduced SQL queries per GraphQL request by {summary['improvement']['sql_queries_reduction_pct']:.2f}% by enabling dataloader batching for category resolution.\\n"
    + f"- Improved GraphQL resolver performance by {summary['improvement']['resolver_latency_reduction_pct']:.2f}% on productCategory and increased throughput by {summary['improvement']['throughput_improvement_pct']:.2f}%.\\n"
)

print(json.dumps(summary, indent=2))
print("\\nSaved:")
print(f"- $REPORT_JSON")
print(f"- $REPORT_MD")
PY
