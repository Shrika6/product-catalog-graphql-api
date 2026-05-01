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
require_cmd k6

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

BASE_URL="${BASE_URL:-http://localhost:8080}"
GRAPHQL_URL="${GRAPHQL_URL:-${BASE_URL}/query}"
MODE="${MODE:-read-heavy}"
OUT_DIR="${OUT_DIR:-loadtest}"
OUT_FILE="${OUT_FILE:-${OUT_DIR}/${MODE}-summary.json}"
JWT_SECRET="${JWT_SECRET:-}"
JWT_SUBJECT="${JWT_SUBJECT:-load-tester}"
JWT_EMAIL="${JWT_EMAIL:-load@test.local}"
JWT_ROLES="${JWT_ROLES:-admin}"
JWT_TTL_SECONDS="${JWT_TTL_SECONDS:-3600}"

mkdir -p "$OUT_DIR"

if [[ -z "$JWT_SECRET" ]]; then
  echo "JWT_SECRET is required. Set it in .env or export JWT_SECRET before running."
  exit 1
fi

curl_graphql() {
  local payload="$1"

  if [[ -n "${BASIC_AUTH_USERNAME:-}" && -n "${BASIC_AUTH_PASSWORD:-}" ]]; then
    curl -sS -u "${BASIC_AUTH_USERNAME}:${BASIC_AUTH_PASSWORD}" \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "$GRAPHQL_URL"
  else
    curl -sS \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "$GRAPHQL_URL"
  fi
}

assert_no_errors() {
  local response="$1"
  local errors_len
  errors_len="$(echo "$response" | jq '.errors | length // 0')"
  if [[ "$errors_len" != "0" ]]; then
    echo "GraphQL returned errors:"
    echo "$response" | jq .
    exit 1
  fi
}

graphql_query() {
  local query="$1"
  local variables_json="${2:-null}"
  jq -cn --arg q "$query" --argjson v "$variables_json" '{query:$q, variables:$v}'
}

generate_jwt() {
  python3 - <<'PY'
import base64, hmac, hashlib, json, os, time

secret = os.environ["JWT_SECRET"].encode()
subject = os.environ.get("JWT_SUBJECT", "load-tester")
email = os.environ.get("JWT_EMAIL", "load@test.local")
roles = [r.strip() for r in os.environ.get("JWT_ROLES", "admin").split(",") if r.strip()]
ttl = int(os.environ.get("JWT_TTL_SECONDS", "3600"))

header = {"alg":"HS256","typ":"JWT"}
now = int(time.time())
payload = {
    "sub": subject,
    "email": email,
    "roles": roles,
    "iat": now,
    "exp": now + ttl,
}

def b64url(d):
    s = json.dumps(d, separators=(",", ":")).encode()
    return base64.urlsafe_b64encode(s).rstrip(b"=")

h = b64url(header)
p = b64url(payload)
sig = base64.urlsafe_b64encode(hmac.new(secret, h+b"."+p, hashlib.sha256).digest()).rstrip(b"=")
print((h+b"."+p+b"."+sig).decode())
PY
}

JWT_TOKEN="$(generate_jwt)"

echo "Generated JWT token for load test subject: ${JWT_SUBJECT}"

CATEGORY_ID="${CATEGORY_ID:-}"
if [[ -z "$CATEGORY_ID" ]]; then
  echo "Fetching an existing category..."
  list_categories_payload="$(graphql_query 'query { categories { id name } }')"
  categories_response="$(curl_graphql "$list_categories_payload")"
  assert_no_errors "$categories_response"
  CATEGORY_ID="$(echo "$categories_response" | jq -r '.data.categories[0].id // empty')"

  if [[ -z "$CATEGORY_ID" ]]; then
    echo "No category found. Creating one..."
    create_category_payload="$(jq -cn \
      --arg q 'mutation CreateCategory($input: CreateCategoryInput!) { createCategory(input: $input) { id name } }' \
      --argjson v '{"input":{"name":"k6-load-category"}}' \
      '{query:$q, variables:$v}')"

    create_category_response="$(
      if [[ -n "${BASIC_AUTH_USERNAME:-}" && -n "${BASIC_AUTH_PASSWORD:-}" ]]; then
        curl -sS -u "${BASIC_AUTH_USERNAME}:${BASIC_AUTH_PASSWORD}" \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer ${JWT_TOKEN}" \
          -d "$create_category_payload" \
          "$GRAPHQL_URL"
      else
        curl -sS \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer ${JWT_TOKEN}" \
          -d "$create_category_payload" \
          "$GRAPHQL_URL"
      fi
    )"

    assert_no_errors "$create_category_response"
    CATEGORY_ID="$(echo "$create_category_response" | jq -r '.data.createCategory.id // empty')"
  fi
fi

if [[ -z "$CATEGORY_ID" ]]; then
  echo "Failed to resolve CATEGORY_ID"
  exit 1
fi

echo "Using CATEGORY_ID=$CATEGORY_ID"

PRODUCT_ID="${PRODUCT_ID:-}"
if [[ -z "$PRODUCT_ID" ]]; then
  echo "Fetching an existing product..."
  list_products_payload="$(graphql_query 'query { products(pagination:{limit:1, offset:0}) { id name } }')"
  products_response="$(curl_graphql "$list_products_payload")"
  assert_no_errors "$products_response"
  PRODUCT_ID="$(echo "$products_response" | jq -r '.data.products[0].id // empty')"

  if [[ -z "$PRODUCT_ID" ]]; then
    echo "No product found. Creating one..."
    create_product_payload="$(jq -cn \
      --arg q 'mutation CreateProduct($input: CreateProductInput!) { createProduct(input: $input) { id name } }' \
      --arg cid "$CATEGORY_ID" \
      --argjson v '{"input":{"name":"k6-seed-product","description":"Created by run_k6_benchmark.sh","price":49.99,"currency":"USD","categoryId":$cid,"stockQuantity":25}}' \
      '{query:$q, variables:$v}')"

    create_product_response="$(
      if [[ -n "${BASIC_AUTH_USERNAME:-}" && -n "${BASIC_AUTH_PASSWORD:-}" ]]; then
        curl -sS -u "${BASIC_AUTH_USERNAME}:${BASIC_AUTH_PASSWORD}" \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer ${JWT_TOKEN}" \
          -d "$create_product_payload" \
          "$GRAPHQL_URL"
      else
        curl -sS \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer ${JWT_TOKEN}" \
          -d "$create_product_payload" \
          "$GRAPHQL_URL"
      fi
    )"

    assert_no_errors "$create_product_response"
    PRODUCT_ID="$(echo "$create_product_response" | jq -r '.data.createProduct.id // empty')"
  fi
fi

if [[ -z "$PRODUCT_ID" ]]; then
  echo "Failed to resolve PRODUCT_ID"
  exit 1
fi

echo "Using PRODUCT_ID=$PRODUCT_ID"

echo "Running k6 mode=$MODE..."

BASE_URL="$BASE_URL" \
MODE="$MODE" \
PRODUCT_ID="$PRODUCT_ID" \
CATEGORY_ID="$CATEGORY_ID" \
JWT_TOKEN="$JWT_TOKEN" \
k6 run --summary-export="$OUT_FILE" loadtest/graphql_load_test.js

echo ""
echo "Benchmark complete."
echo "Summary file: $OUT_FILE"
echo "Key metrics:"
jq '.metrics.http_reqs.values.rate as $qps |
    .metrics.http_req_duration.values["p(95)"] as $p95 |
    .metrics.http_req_duration.values["p(99)"] as $p99 |
    .metrics.http_req_failed.values.rate as $err |
    {qps:$qps, p95_ms:$p95, p99_ms:$p99, error_rate:$err}' "$OUT_FILE"
