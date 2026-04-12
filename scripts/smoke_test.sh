#!/usr/bin/env bash

set -euo pipefail

API_URL="${API_URL:-http://localhost:8080/query}"
AUTH_USER="${BASIC_AUTH_USERNAME:-}"
AUTH_PASS="${BASIC_AUTH_PASSWORD:-}"

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Required command not found: ${cmd}"
    exit 1
  fi
}

curl_graphql() {
  local query="$1"
  local payload
  payload="$(jq -cn --arg q "${query}" '{query: $q}')"

  if [[ -n "${AUTH_USER}" && -n "${AUTH_PASS}" ]]; then
    curl -sS -u "${AUTH_USER}:${AUTH_PASS}" \
      -H "Content-Type: application/json" \
      -d "${payload}" \
      "${API_URL}"
  else
    curl -sS \
      -H "Content-Type: application/json" \
      -d "${payload}" \
      "${API_URL}"
  fi
}

assert_no_graphql_errors() {
  local response="$1"
  local error_count
  error_count="$(echo "${response}" | jq '.errors | length // 0')"
  if [[ "${error_count}" != "0" ]]; then
    echo "GraphQL request failed:"
    echo "${response}" | jq .
    exit 1
  fi
}

require_cmd curl
require_cmd jq

echo "1) Creating category..."
CREATE_CATEGORY_RESPONSE="$(curl_graphql 'mutation { createCategory(input:{name:"Smoke Category"}) { id name } }')"
echo "${CREATE_CATEGORY_RESPONSE}"
assert_no_graphql_errors "${CREATE_CATEGORY_RESPONSE}"

CATEGORY_ID="$(echo "${CREATE_CATEGORY_RESPONSE}" | jq -r '.data.createCategory.id // empty')"
if [[ -z "${CATEGORY_ID}" ]]; then
  echo "Failed to parse category id."
  exit 1
fi

echo "2) Creating product..."
CREATE_PRODUCT_RESPONSE="$(curl_graphql "mutation { createProduct(input:{name:\"Smoke Product\", description:\"smoke run\", price:19.99, currency:\"USD\", categoryId:\"${CATEGORY_ID}\", stockQuantity:10}) { id name category { id name } } }")"
echo "${CREATE_PRODUCT_RESPONSE}"
assert_no_graphql_errors "${CREATE_PRODUCT_RESPONSE}"

PRODUCT_ID="$(echo "${CREATE_PRODUCT_RESPONSE}" | jq -r '.data.createProduct.id // empty')"
if [[ -z "${PRODUCT_ID}" ]]; then
  echo "Failed to parse product id."
  exit 1
fi

echo "3) Querying products with filter/pagination/sorting..."
PRODUCTS_RESPONSE="$(curl_graphql 'query { products(filter:{nameSearch:"Smoke"}, pagination:{limit:10, offset:0}, sorting:{field:PRICE, order:ASC}) { id name price currency } }')"
echo "${PRODUCTS_RESPONSE}"
assert_no_graphql_errors "${PRODUCTS_RESPONSE}"
echo

echo "4) Querying category with nested products..."
CATEGORY_QUERY_RESPONSE="$(curl_graphql "query { category(id:\"${CATEGORY_ID}\") { id name products(limit:20, offset:0) { id name price category { id name } } } }")"
echo "${CATEGORY_QUERY_RESPONSE}"
assert_no_graphql_errors "${CATEGORY_QUERY_RESPONSE}"
echo

echo "5) Validation check (invalid UUID)..."
INVALID_UUID_RESPONSE="$(curl_graphql 'query { category(id:"not-a-uuid") { id } }')"
echo "${INVALID_UUID_RESPONSE}"
if [[ "$(echo "${INVALID_UUID_RESPONSE}" | jq -r '.errors[0].extensions.code // empty')" != "INVALID_ARGUMENT" ]]; then
  echo "Expected INVALID_ARGUMENT for invalid UUID test."
  exit 1
fi
echo

echo "6) Deleting product..."
DELETE_RESPONSE="$(curl_graphql "mutation { deleteProduct(id:\"${PRODUCT_ID}\") }")"
echo "${DELETE_RESPONSE}"
assert_no_graphql_errors "${DELETE_RESPONSE}"
if [[ "$(echo "${DELETE_RESPONSE}" | jq -r '.data.deleteProduct // false')" != "true" ]]; then
  echo "Expected deleteProduct to return true."
  exit 1
fi
echo

echo "Smoke test finished."
