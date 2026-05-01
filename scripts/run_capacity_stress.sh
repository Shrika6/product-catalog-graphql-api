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

require_cmd k6
require_cmd jq
require_cmd docker
require_cmd python3

BASE_URL="${BASE_URL:-http://localhost:8080}"
PRODUCT_ID="${PRODUCT_ID:-}"
RATES="${RATES:-50 100 150 200 250 300 350 400}"
STEP_DURATION="${STEP_DURATION:-2m}"
PRE_VUS="${PRE_VUS:-80}"
MAX_VUS="${MAX_VUS:-800}"
REPORT_DIR="${REPORT_DIR:-loadtest/capacity}"
SUMMARY_FILE="${SUMMARY_FILE:-${REPORT_DIR}/capacity-summary.json}"
METHODOLOGY_FILE="${METHODOLOGY_FILE:-${REPORT_DIR}/methodology.md}"
APP_CONTAINER="${APP_CONTAINER:-}"

mkdir -p "$REPORT_DIR"

if [[ -z "${APP_CONTAINER}" ]]; then
  APP_CONTAINER="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E 'product-catalog.*api|graphql.*api|api' | head -n1 || true)"
fi

if [[ -z "${APP_CONTAINER}" ]]; then
  echo "Warning: could not auto-detect app container; CPU/memory sampling will be skipped."
else
  echo "Using APP_CONTAINER=$APP_CONTAINER"
fi

if [[ -z "$PRODUCT_ID" ]]; then
  echo "PRODUCT_ID not provided, attempting to fetch one..."
  response="$(curl -sS -H 'Content-Type: application/json' -d '{"query":"query { products(pagination:{limit:1, offset:0}) { id } }"}' "${BASE_URL}/query")"
  PRODUCT_ID="$(echo "$response" | jq -r '.data.products[0].id // empty')"
fi

if [[ -z "$PRODUCT_ID" ]]; then
  echo "Could not resolve PRODUCT_ID. Seed at least one product first."
  exit 1
fi

echo "Using PRODUCT_ID=$PRODUCT_ID"

collect_docker_stats() {
  local out_file="$1"
  local stop_file="$2"

  if [[ -z "${APP_CONTAINER}" ]]; then
    return 0
  fi

  while [[ ! -f "$stop_file" ]]; do
    docker stats --no-stream --format '{{.CPUPerc}},{{.MemUsage}},{{.MemPerc}}' "$APP_CONTAINER" >> "$out_file" 2>/dev/null || true
    sleep 1
  done
}

python3 - <<'PY' "$METHODOLOGY_FILE"
import sys
from pathlib import Path

Path(sys.argv[1]).write_text(
"""# Capacity Stress Methodology

1. Run stepped load tests at increasing target arrival rates (RPS).
2. For each step, collect:
   - throughput (http_reqs rate)
   - p95/p99 latency
   - error rate
   - max VUs (proxy for concurrent clients supported)
   - CPU/memory samples from Docker stats
3. Mark a step as unstable when:
   - error rate >= 2% OR
   - p95 > 500ms OR
   - p99 > 1000ms
4. Detect saturation knee where p95 latency increases sharply (>50%) vs previous step.
5. Report:
   - maximum stable QPS
   - maximum concurrent clients (max VUs in stable region)
   - CPU/memory at saturation step
   - throughput before degradation
""")
PY

results_json='[]'
prev_p95=0
saturation_detected=0

for rate in $RATES; do
  echo "Running step TARGET_RPS=$rate ..."

  step_summary="${REPORT_DIR}/step-${rate}.json"
  stats_csv="${REPORT_DIR}/step-${rate}-docker-stats.csv"
  stop_flag="${REPORT_DIR}/.step-${rate}.stop"
  rm -f "$stats_csv" "$stop_flag"

  collect_docker_stats "$stats_csv" "$stop_flag" &
  stats_pid=$!

  BASE_URL="$BASE_URL" \
  PRODUCT_ID="$PRODUCT_ID" \
  TARGET_RPS="$rate" \
  DURATION="$STEP_DURATION" \
  PRE_VUS="$PRE_VUS" \
  MAX_VUS="$MAX_VUS" \
  k6 run --summary-export="$step_summary" loadtest/capacity_stress.js >/dev/null

  touch "$stop_flag"
  wait "$stats_pid" || true

  qps="$(jq '.metrics.http_reqs.values.rate // .metrics.http_reqs.rate // 0' "$step_summary")"
  p95="$(jq '.metrics.http_req_duration.values["p(95)"] // .metrics.http_req_duration["p(95)"] // 0' "$step_summary")"
  p99="$(jq '.metrics.http_req_duration.values["p(99)"] // .metrics.http_req_duration["p(99)"] // 0' "$step_summary")"
  err_rate="$(jq '(
      .metrics.http_req_failed.values.rate
      // .metrics.http_req_failed.rate
      // .metrics.http_req_failed.value
      // (
        if ((.metrics.checks.fails // 0) + (.metrics.checks.passes // 0)) > 0
        then (.metrics.checks.fails // 0) / ((.metrics.checks.fails // 0) + (.metrics.checks.passes // 0))
        else 0
        end
      )
      // 0
    )' "$step_summary")"
  vus_max="$(jq '.metrics.vus_max.values.max // .metrics.vus_max.max // .metrics.vus_max.value // 0' "$step_summary")"

  cpu_max="0"
  mem_max_pct="0"
  mem_peak="n/a"
  if [[ -s "$stats_csv" ]]; then
    cpu_max="$(awk -F',' '{gsub(/%/,"",$1); if($1+0>max) max=$1+0} END{print max+0}' "$stats_csv")"
    mem_max_pct="$(awk -F',' '{gsub(/%/,"",$3); if($3+0>max) max=$3+0} END{print max+0}' "$stats_csv")"
    mem_peak="$(awk -F',' '{gsub(/^ +| +$/,"",$2); print $2}' "$stats_csv" | tail -n1)"
  fi

  stable="true"
  if python3 - <<PY
p95=float("$p95")
p99=float("$p99")
err=float("$err_rate")
import sys
sys.exit(0 if (err < 0.02 and p95 < 500 and p99 < 1000) else 1)
PY
  then
    stable="true"
  else
    stable="false"
  fi

  knee="false"
  if python3 - <<PY
prev=float("$prev_p95")
cur=float("$p95")
import sys
if prev <= 0:
    sys.exit(1)
ratio=(cur-prev)/prev
sys.exit(0 if ratio > 0.5 else 1)
PY
  then
    knee="true"
  fi

  prev_p95="$p95"

  step_entry="$(jq -cn \
    --argjson rate "$rate" \
    --argjson qps "$qps" \
    --argjson p95 "$p95" \
    --argjson p99 "$p99" \
    --argjson err "$err_rate" \
    --argjson vus "$vus_max" \
    --argjson cpu "$cpu_max" \
    --argjson mempct "$mem_max_pct" \
    --arg mempeak "$mem_peak" \
    --arg stable "$stable" \
    --arg knee "$knee" \
    '{target_rps:$rate,qps:$qps,p95_ms:$p95,p99_ms:$p99,error_rate:$err,max_vus:$vus,max_cpu_pct:$cpu,max_mem_pct:$mempct,mem_peak:$mempeak,stable:($stable=="true"),latency_knee:($knee=="true")}')"

  results_json="$(jq -cn --argjson arr "$results_json" --argjson item "$step_entry" '$arr + [$item]')"
done

python3 - <<PY
import json
from pathlib import Path

steps = json.loads('''$results_json''')
stable_steps = [s for s in steps if s.get('stable')]

max_stable_qps = max((s['qps'] for s in stable_steps), default=0)
max_concurrent_clients = max((s['max_vus'] for s in stable_steps), default=0)
throughput_before_degrade = max_stable_qps

saturation_step = None
for s in steps:
    if (not s.get('stable')) or s.get('latency_knee'):
        saturation_step = s
        break
if saturation_step is None and steps:
    saturation_step = steps[-1]

summary = {
    'steps': steps,
    'capacity': {
        'maximum_concurrent_clients_supported': max_concurrent_clients,
        'max_qps_before_saturation': max_stable_qps,
        'throughput_before_degrades': throughput_before_degrade,
        'saturation_step': saturation_step,
        'cpu_usage_at_saturation_pct': saturation_step.get('max_cpu_pct', 0) if saturation_step else 0,
        'memory_usage_at_saturation_pct': saturation_step.get('max_mem_pct', 0) if saturation_step else 0,
    },
    'resume_bullets': [
        f"Sustained up to {max_stable_qps:.2f} req/s before saturation under stepped GraphQL load.",
        f"Supported approximately {max_concurrent_clients:.0f} concurrent virtual users in stable operation.",
        f"Observed saturation around target {saturation_step.get('target_rps', 0)} rps with CPU {saturation_step.get('max_cpu_pct', 0):.2f}% and memory {saturation_step.get('max_mem_pct', 0):.2f}%.",
        f"Latency knee detected near p95={saturation_step.get('p95_ms', 0):.2f} ms and error rate={saturation_step.get('error_rate', 0)*100:.2f}%.",
    ]
}

Path('$SUMMARY_FILE').write_text(json.dumps(summary, indent=2))
print(json.dumps(summary, indent=2))
PY

echo "Saved capacity summary: $SUMMARY_FILE"
echo "Saved methodology: $METHODOLOGY_FILE"
