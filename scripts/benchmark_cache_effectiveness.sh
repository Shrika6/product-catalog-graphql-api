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

BASE_URL="${BASE_URL:-http://localhost:8080}"
GRAPHQL_URL="${GRAPHQL_URL:-${BASE_URL}/query}"
METRICS_URL="${METRICS_URL:-${BASE_URL}/metrics}"
HIT_REQUESTS="${HIT_REQUESTS:-200}"
MISS_REQUESTS="${MISS_REQUESTS:-200}"
REPORT_DIR="${REPORT_DIR:-loadtest}"
REPORT_JSON="${REPORT_JSON:-${REPORT_DIR}/cache-effectiveness-report.json}"
REPORT_MD="${REPORT_MD:-${REPORT_DIR}/cache-effectiveness-resume.md}"
PRODUCT_ID="${PRODUCT_ID:-}"

mkdir -p "$REPORT_DIR"

graphql_post() {
  local payload="$1"
  if [[ -n "${BASIC_AUTH_USERNAME:-}" && -n "${BASIC_AUTH_PASSWORD:-}" ]]; then
    curl -sS -u "${BASIC_AUTH_USERNAME}:${BASIC_AUTH_PASSWORD}" \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "$GRAPHQL_URL"
  else
    curl -sS -H "Content-Type: application/json" -d "$payload" "$GRAPHQL_URL"
  fi
}

snapshot_metrics() {
  local out_file="$1"
  curl -sS "$METRICS_URL" > "$out_file"
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

safe_div_pct() {
  python3 - <<PY
num=float("$1")
den=float("$2")
print(0 if den == 0 else (num/den)*100)
PY
}

safe_avg_ms() {
  python3 - <<PY
sum_v=float("$1")
count_v=float("$2")
print(0 if count_v == 0 else (sum_v/count_v)*1000)
PY
}

if [[ -z "$PRODUCT_ID" ]]; then
  echo "PRODUCT_ID not provided, fetching one from API..."
  payload='{"query":"query { products(pagination:{limit:1, offset:0}) { id name } }"}'
  response="$(graphql_post "$payload")"
  PRODUCT_ID="$(echo "$response" | jq -r '.data.products[0].id // empty')"
fi

if [[ -z "$PRODUCT_ID" ]]; then
  echo "Could not resolve PRODUCT_ID. Seed at least one product and retry."
  exit 1
fi

echo "Using PRODUCT_ID=$PRODUCT_ID"

echo "Warming cache for product by ID..."
warm_payload="$(jq -cn --arg id "$PRODUCT_ID" '{query:"query Product($id: ID!) { product(id: $id) { id name } }", variables:{id:$id}}')"
graphql_post "$warm_payload" >/dev/null

BASE_METRICS="$(mktemp)"
AFTER_HITS_METRICS="$(mktemp)"
AFTER_MISSES_METRICS="$(mktemp)"
trap 'rm -f "$BASE_METRICS" "$AFTER_HITS_METRICS" "$AFTER_MISSES_METRICS"' EXIT

snapshot_metrics "$BASE_METRICS"

echo "Running hit phase (${HIT_REQUESTS} requests)..."
for _ in $(seq 1 "$HIT_REQUESTS"); do
  graphql_post "$warm_payload" >/dev/null || true
done

snapshot_metrics "$AFTER_HITS_METRICS"

echo "Running miss phase (${MISS_REQUESTS} requests with random UUIDs)..."
for _ in $(seq 1 "$MISS_REQUESTS"); do
  random_id="$(python3 - <<'PY'
import uuid
print(uuid.uuid4())
PY
)"
  miss_payload="$(jq -cn --arg id "$random_id" '{query:"query Product($id: ID!) { product(id: $id) { id name } }", variables:{id:$id}}')"
  graphql_post "$miss_payload" >/dev/null || true
done

snapshot_metrics "$AFTER_MISSES_METRICS"

KEY_HIT='cache_operations_total{cache="redis",operation="get_product_by_id",result="hit"}'
KEY_MISS='cache_operations_total{cache="redis",operation="get_product_by_id",result="miss"}'
KEY_HIT_SUM='cache_operation_duration_seconds_sum{cache="redis",operation="get_product_by_id",result="hit"}'
KEY_HIT_COUNT='cache_operation_duration_seconds_count{cache="redis",operation="get_product_by_id",result="hit"}'
KEY_MISS_SUM='cache_operation_duration_seconds_sum{cache="redis",operation="get_product_by_id",result="miss"}'
KEY_MISS_COUNT='cache_operation_duration_seconds_count{cache="redis",operation="get_product_by_id",result="miss"}'
KEY_DB_COUNT='db_query_duration_seconds_count{repository="product_repository",method="GetByID"}'

hit_base="$(metric_value "$BASE_METRICS" "$KEY_HIT")"
hit_after_miss="$(metric_value "$AFTER_MISSES_METRICS" "$KEY_HIT")"
miss_base="$(metric_value "$BASE_METRICS" "$KEY_MISS")"
miss_after_miss="$(metric_value "$AFTER_MISSES_METRICS" "$KEY_MISS")"

hit_delta="$(delta "$hit_base" "$hit_after_miss")"
miss_delta="$(delta "$miss_base" "$miss_after_miss")"
total_lookups="$(python3 - <<PY
h=float("$hit_delta")
m=float("$miss_delta")
print(h+m)
PY
)"

hit_ratio_pct="$(safe_div_pct "$hit_delta" "$total_lookups")"
miss_ratio_pct="$(safe_div_pct "$miss_delta" "$total_lookups")"

hit_sum_base="$(metric_value "$BASE_METRICS" "$KEY_HIT_SUM")"
hit_sum_after="$(metric_value "$AFTER_MISSES_METRICS" "$KEY_HIT_SUM")"
hit_count_base="$(metric_value "$BASE_METRICS" "$KEY_HIT_COUNT")"
hit_count_after="$(metric_value "$AFTER_MISSES_METRICS" "$KEY_HIT_COUNT")"

miss_sum_base="$(metric_value "$BASE_METRICS" "$KEY_MISS_SUM")"
miss_sum_after="$(metric_value "$AFTER_MISSES_METRICS" "$KEY_MISS_SUM")"
miss_count_base="$(metric_value "$BASE_METRICS" "$KEY_MISS_COUNT")"
miss_count_after="$(metric_value "$AFTER_MISSES_METRICS" "$KEY_MISS_COUNT")"

hit_sum_delta="$(delta "$hit_sum_base" "$hit_sum_after")"
hit_count_delta="$(delta "$hit_count_base" "$hit_count_after")"
miss_sum_delta="$(delta "$miss_sum_base" "$miss_sum_after")"
miss_count_delta="$(delta "$miss_count_base" "$miss_count_after")"

avg_hit_latency_ms="$(safe_avg_ms "$hit_sum_delta" "$hit_count_delta")"
avg_miss_latency_ms="$(safe_avg_ms "$miss_sum_delta" "$miss_count_delta")"

latency_improvement_pct="$(python3 - <<PY
hit=float("$avg_hit_latency_ms")
miss=float("$avg_miss_latency_ms")
print(0 if miss == 0 else ((miss-hit)/miss)*100)
PY
)"

db_count_base="$(metric_value "$BASE_METRICS" "$KEY_DB_COUNT")"
db_count_after="$(metric_value "$AFTER_MISSES_METRICS" "$KEY_DB_COUNT")"
db_queries_actual="$(delta "$db_count_base" "$db_count_after")"

db_offload_pct="$(safe_div_pct "$hit_delta" "$total_lookups")"

cat > "$REPORT_JSON" <<JSON
{
  "benchmark": {
    "base_url": "$BASE_URL",
    "product_id": "$PRODUCT_ID",
    "hit_requests": $HIT_REQUESTS,
    "miss_requests": $MISS_REQUESTS
  },
  "metrics": {
    "cache_hit_ratio_pct": $hit_ratio_pct,
    "cache_miss_ratio_pct": $miss_ratio_pct,
    "avg_cache_hit_latency_ms": $avg_hit_latency_ms,
    "avg_cache_miss_latency_ms": $avg_miss_latency_ms,
    "latency_improvement_pct": $latency_improvement_pct,
    "db_offload_pct": $db_offload_pct,
    "db_queries_actual": $db_queries_actual,
    "cache_hits": $hit_delta,
    "cache_misses": $miss_delta,
    "total_cache_lookups": $total_lookups
  },
  "formulas": {
    "hit_rate_pct": "cache_hits / (cache_hits + cache_misses) * 100",
    "latency_reduction_pct": "(avg_miss_latency_ms - avg_hit_latency_ms) / avg_miss_latency_ms * 100",
    "db_offload_pct": "cache_hits / (cache_hits + cache_misses) * 100"
  }
}
JSON

cat > "$REPORT_MD" <<MD
# Cache Effectiveness Benchmark

- Cache hit ratio: $(printf "%.2f" "$hit_ratio_pct")%
- Avg latency with cache hit: $(printf "%.3f" "$avg_hit_latency_ms") ms
- Avg latency with cache miss: $(printf "%.3f" "$avg_miss_latency_ms") ms
- Latency improvement from caching: $(printf "%.2f" "$latency_improvement_pct")%
- Database offload due to caching: $(printf "%.2f" "$db_offload_pct")%

## Resume-ready bullets

- Improved product read path efficiency with Redis, achieving a $(printf "%.2f" "$hit_ratio_pct")% cache hit ratio under benchmark load.
- Reduced read latency by $(printf "%.2f" "$latency_improvement_pct")% (avg hit: $(printf "%.3f" "$avg_hit_latency_ms") ms vs miss: $(printf "%.3f" "$avg_miss_latency_ms") ms).
- Offloaded approximately $(printf "%.2f" "$db_offload_pct")% of product-by-id lookups from PostgreSQL through effective cache utilization.

## Formula references

- Hit rate % = cache_hits / (cache_hits + cache_misses) * 100
- Latency reduction % = (avg_miss_latency_ms - avg_hit_latency_ms) / avg_miss_latency_ms * 100
- DB offload % = cache_hits / (cache_hits + cache_misses) * 100
MD

echo "Cache benchmark complete."
echo "JSON report: $REPORT_JSON"
echo "Resume report: $REPORT_MD"
cat "$REPORT_JSON" | jq .
