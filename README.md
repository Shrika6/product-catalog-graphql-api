# GraphQL Product Catalog API (Go + gqlgen)

Production-oriented GraphQL Product Catalog API built with Go, gqlgen, PostgreSQL, GORM, and Docker.

## Features

- Clean architecture structure (`cmd`, `internal`, `pkg`, `migrations`)
- GraphQL API with gqlgen
- PostgreSQL persistence with GORM repository layer
- Migration support via `golang-migrate`
- Product filtering, pagination, and sorting
- Structured logs (`slog`)
- Request context timeouts and graceful shutdown
- DB connection pooling
- GraphQL-friendly error responses with error codes
- Optional Basic Auth middleware
- JWT-based auth to protect mutations
- Dataloader for category lookups (`Product.category`) to avoid N+1
- Redis caching for product list and product-by-ID (5 minute TTL)
- Accessible React UI served from `/`
- Service-layer unit tests

## Project Structure

```text
product-catalog-graphql-api/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── graph/
│   │   ├── dataloader/
│   │   ├── generated/
│   │   ├── model/
│   │   ├── resolver/
│   │   └── schema.graphqls
│   ├── middleware/
│   ├── model/
│   ├── repository/
│   ├── service/
│   └── ui/
├── migrations/
│   ├── 000001_create_catalog_tables.up.sql
│   └── 000001_create_catalog_tables.down.sql
├── pkg/
│   ├── config/
│   ├── db/
│   ├── errors/
│   └── logger/
├── tools/
│   └── tools.go
├── .env.example
├── Dockerfile
├── docker-compose.yml
├── gqlgen.yml
├── Makefile
└── README.md
```

## GraphQL Schema

Defined in `internal/graph/schema.graphqls`.

### Queries

- `products(filter, pagination, sorting)`
- `product(id: ID!)`
- `categories`
- `category(id: ID!)`

### Mutations

- `createProduct(input)`
- `updateProduct(id, input)`
- `deleteProduct(id)`
- `createCategory(input)`

### Filtering

Products support:

- `minPrice` / `maxPrice`
- `categoryId`
- `nameSearch` (PostgreSQL full-text search across name + description)

### Pagination

- limit/offset via `PaginationInput`

### Sorting

- `NAME`, `PRICE`, `CREATED_AT`, `UPDATED_AT`, `STOCK_QUANTITY`
- order: `ASC` or `DESC`

## Local Setup

### 1) Configure environment

```bash
cp .env.example .env
```

If you want caching locally, ensure Redis is running and set:

```env
REDIS_ADDR=localhost:6379
```

### 2) Start PostgreSQL

```bash
docker compose up -d postgres
```

### 3) Run migrations

Install `golang-migrate` CLI, then run:

```bash
export $(grep -v '^#' .env | xargs)
make migrate-up
```

### 4) Run API

```bash
make run
```

GraphQL endpoint: `http://localhost:8080/query`  
UI: `http://localhost:8080/`  
Playground (non-production): `http://localhost:8080/playground`

## Docker Run

```bash
docker compose up --build
```

This starts:

- `app` on `:8080`
- `postgres` on `:5432`
- `redis` on `:6379`

## Example GraphQL Operations

### Create category

```graphql
mutation {
  createCategory(input: { name: "Electronics" }) {
    id
    name
    createdAt
  }
}
```

### Create product

```graphql
mutation {
  createProduct(
    input: {
      name: "Mechanical Keyboard"
      description: "Hot-swappable 75% keyboard"
      price: 129.99
      currency: "USD"
      categoryId: "<CATEGORY_UUID>"
      stockQuantity: 45
    }
  ) {
    id
    name
    price
    currency
    stockQuantity
    category {
      id
      name
    }
  }
}
```

### Fetch products with filters + pagination + sorting

```graphql
query {
  products(
    filter: { minPrice: 20, maxPrice: 200, nameSearch: "keyboard" }
    pagination: { limit: 10, offset: 0 }
    sorting: { field: PRICE, order: ASC }
  ) {
    id
    name
    price
    currency
    categoryId
    stockQuantity
  }
}
```

### Fetch products by category

```graphql
query {
  category(id: "<CATEGORY_UUID>") {
    id
    name
    products(limit: 20, offset: 0) {
      id
      name
      price
      category {
        id
        name
      }
    }
  }
}
```

## Validation & Error Handling

Business validation lives in `internal/service`.

GraphQL errors include machine-readable extension code:

- `INVALID_ARGUMENT`
- `NOT_FOUND`
- `INTERNAL`
- `UNAUTHORIZED`

## Accessible UI

The app includes a minimal React UI with accessibility-first defaults:

- Semantic landmarks (`header`, `nav`, `main`, section regions)
- Skip link and keyboard-friendly tab order
- ARIA labels and live status announcements
- Focus management after search results load
- Meaningful image alt text (`Product Catalog logo`)
- Native interactive elements (`button`, `input`, `select`) instead of clickable `div`s

Alt text convention:

- Informative images: concise purpose-driven `alt` text
- Decorative images: empty alt (`alt=\"\"`) and hidden from assistive tech when appropriate

Accessibility audit:

- Lighthouse accessibility score: **100/100** (April 17, 2026)

## Basic Auth (Optional)

Set both env vars to enable auth on `/query`:

```env
BASIC_AUTH_USERNAME=admin
BASIC_AUTH_PASSWORD=change-me
```

## JWT Authentication

Mutations require a valid JWT (queries remain public). Configure:

```env
JWT_SECRET=change-me
JWT_ISSUER=
JWT_AUDIENCE=
```

Send the token in the `Authorization` header:

```bash
curl -sS -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -d '{"query":"mutation { createCategory(input:{name:\"Secure Category\"}) { id name } }"}' \
  http://localhost:8080/query
```

If the token is missing or invalid, mutations return:

```
{"errors":[{"message":"unauthorized","extensions":{"code":"UNAUTHORIZED"}}]}
```

## Redis Cache (Optional)

Caching is enabled when `REDIS_ADDR` is set. If Redis is unavailable, the API will log a warning and continue without cache.

Cache policy:

- Product list queries (`products`) cached for 5 minutes
- Product by ID (`product(id: ...)`) cached for 5 minutes
- Cache invalidation on create/update/delete (list version bump + product key delete)

## Development Commands

```bash
make gqlgen   # regenerate gqlgen files after schema changes
make test     # run unit tests
make build    # build binary
make smoke-test # run API smoke test script
```

## Observability (Prometheus)

The API exports Prometheus metrics at:

- `GET /metrics`

### What is instrumented

- HTTP middleware:
  - request count, request latency, concurrent in-flight requests
- GraphQL resolvers:
  - per-operation counters, errors, and latency histograms
  - operation labels like `products`, `product`, `createProduct`, `updateProduct`, `deleteProduct`
- Repository/database calls:
  - per-repository-method query latency histograms
- Redis cache operations:
  - cache hit/miss/error/success counters
  - cache operation latency histograms
  - cache miss penalty histogram (fallback latency after miss)

### Example Prometheus scrape config

```yaml
scrape_configs:
  - job_name: product-catalog-api
    static_configs:
      - targets: ["localhost:8080"]
    metrics_path: /metrics
```

### Example PromQL queries

QPS (GraphQL operations):

```promql
sum by (operation) (rate(graphql_operations_total[1m]))
```

Resolver average latency (seconds):

```promql
sum(rate(graphql_resolver_duration_seconds_sum[5m])) by (operation)
/
sum(rate(graphql_resolver_duration_seconds_count[5m])) by (operation)
```

Resolver p50/p95/p99 latency:

```promql
histogram_quantile(0.50, sum(rate(graphql_resolver_duration_seconds_bucket[5m])) by (operation, le))
```

```promql
histogram_quantile(0.95, sum(rate(graphql_resolver_duration_seconds_bucket[5m])) by (operation, le))
```

```promql
histogram_quantile(0.99, sum(rate(graphql_resolver_duration_seconds_bucket[5m])) by (operation, le))
```

Database query latency p95:

```promql
histogram_quantile(0.95, sum(rate(db_query_duration_seconds_bucket[5m])) by (repository, method, le))
```

Cache hit rate:

```promql
sum(rate(cache_operations_total{result="hit"}[5m]))
/
sum(rate(cache_operations_total{result=~"hit|miss"}[5m]))
```

Cache miss rate:

```promql
sum(rate(cache_operations_total{result="miss"}[5m]))
/
sum(rate(cache_operations_total{result=~"hit|miss"}[5m]))
```

Cache miss penalty p95:

```promql
histogram_quantile(0.95, sum(rate(cache_miss_penalty_seconds_bucket[5m])) by (operation, le))
```

Error rate (GraphQL):

```promql
sum(rate(graphql_errors_total[5m])) by (operation)
/
sum(rate(graphql_operations_total[5m])) by (operation)
```

Concurrent requests in flight:

```promql
http_inflight_requests
```

## Load Testing (k6)

Load-test assets live in `loadtest/`:

- k6 script: `loadtest/graphql_load_test.js`
- Sample payloads:
  - `loadtest/payloads/products_query.json`
  - `loadtest/payloads/product_query.json`
  - `loadtest/payloads/create_product_mutation.json`

### Scenarios included

- Sustained read-heavy workload (`MODE=read-heavy`)
  - ~70% `products`, ~30% `product`
- Mixed read/write workload (`MODE=mixed`)
  - ~75% reads, ~25% writes (`createProduct` / `updateProduct`)
- Increasing ramp test until saturation (`MODE=ramp`)
  - step-up arrival rate to identify throughput degradation point

### Prerequisites

- k6 installed (`k6 version`)
- Running API at `BASE_URL` (default: `http://localhost:8080`)
- Seeded `PRODUCT_ID` for read queries
- For mixed mode:
  - `CATEGORY_ID`
  - `JWT_TOKEN` (mutations are JWT-protected)

### Run commands

Read-heavy sustained test:

```bash
PRODUCT_ID=<PRODUCT_UUID> \
MODE=read-heavy \
k6 run --summary-export=loadtest/read-heavy-summary.json loadtest/graphql_load_test.js
```

Mixed read/write test:

```bash
PRODUCT_ID=<PRODUCT_UUID> \
CATEGORY_ID=<CATEGORY_UUID> \
JWT_TOKEN=<JWT_TOKEN> \
MODE=mixed \
k6 run --summary-export=loadtest/mixed-summary.json loadtest/graphql_load_test.js
```

Ramp-until-saturation test:

```bash
PRODUCT_ID=<PRODUCT_UUID> \
MODE=ramp \
k6 run --summary-export=loadtest/ramp-summary.json loadtest/graphql_load_test.js
```

Optional custom API URL:

```bash
BASE_URL=http://localhost:8080 PRODUCT_ID=<PRODUCT_UUID> MODE=read-heavy \
k6 run --summary-export=loadtest/read-heavy-summary.json loadtest/graphql_load_test.js
```

### Extract final metrics for resume bullets

From k6 CLI output and exported summary JSON:

- Maximum stable QPS:
  - from ramp scenario, choose highest stage before p95/p99 or error rate breach
- p95 / p99 latency:
  - `http_req_duration` `p(95)` and `p(99)`
- Error rate:
  - `http_req_failed` rate
- Throughput before degradation:
  - `http_reqs` rate at last healthy ramp stage

Quick extraction helpers (from summary JSON):

```bash
jq '.metrics.http_reqs.values.rate' loadtest/ramp-summary.json
jq '.metrics.http_req_duration.values["p(95)"]' loadtest/ramp-summary.json
jq '.metrics.http_req_duration.values["p(99)"]' loadtest/ramp-summary.json
jq '.metrics.http_req_failed.values.rate' loadtest/ramp-summary.json
```

Suggested degradation threshold (adjust to your SLA):

- p95 > 500 ms OR p99 > 1000 ms OR error rate > 2%

### Dataloader before/after benchmark

Use this script to benchmark GraphQL resolver behavior with dataloader disabled vs enabled:

```bash
./scripts/benchmark_dataloader_impact.sh
```

What it measures:

- SQL queries executed per request (`X-SQL-Statements` response header)
- Resolver latency before/after (`productCategory` resolver avg from Prometheus histogram deltas)
- Percent reduction in DB calls (Category `GetByID`)
- Throughput improvement (requests per second)

Outputs:

- `loadtest/dataloader-before.json`
- `loadtest/dataloader-after.json`
- `loadtest/dataloader-impact-report.json`
- `loadtest/dataloader-impact-resume.md`

Formulas:

- SQL reduction % = `(before_avg_sql_per_req - after_avg_sql_per_req) / before_avg_sql_per_req * 100`
- DB call reduction % = `(before_db_calls - after_db_calls) / before_db_calls * 100`
- Resolver latency reduction % = `(before_resolver_ms - after_resolver_ms) / before_resolver_ms * 100`
- Throughput improvement % = `(after_rps - before_rps) / before_rps * 100`

Note:

- `DATALOADER_ENABLED=true` (default) can be toggled for controlled benchmarking.
- The benchmark query intentionally includes nested `category` under `products` to surface N+1 behavior.

### Cache effectiveness benchmark (Redis)

Use the automated script:

```bash
./scripts/benchmark_cache_effectiveness.sh
```

Optional overrides:

```bash
BASE_URL=http://localhost:8080 \
PRODUCT_ID=<PRODUCT_UUID> \
HIT_REQUESTS=300 \
MISS_REQUESTS=300 \
./scripts/benchmark_cache_effectiveness.sh
```

Outputs:

- JSON metrics report: `loadtest/cache-effectiveness-report.json`
- Resume-ready summary: `loadtest/cache-effectiveness-resume.md`

Measured values:

- cache hit ratio
- average latency with cache hit
- average latency with cache miss
- percent latency improvement from caching
- database load reduction (DB offload) due to caching

Formulas used:

- Hit rate % = `cache_hits / (cache_hits + cache_misses) * 100`
- Latency reduction % = `(avg_miss_latency_ms - avg_hit_latency_ms) / avg_miss_latency_ms * 100`
- DB offload % = `cache_hits / (cache_hits + cache_misses) * 100`

### Capacity stress test (saturation limits)

Use the stepped stress runner:

```bash
./scripts/run_capacity_stress.sh
```

Optional overrides:

```bash
BASE_URL=http://localhost:8080 \
PRODUCT_ID=<PRODUCT_UUID> \
RATES="50 100 150 200 250 300 350 400 450" \
STEP_DURATION=2m \
PRE_VUS=80 \
MAX_VUS=800 \
APP_CONTAINER=product-catalog-api \
./scripts/run_capacity_stress.sh
```

What this measures per step:

- target load (`TARGET_RPS`)
- achieved throughput (`http_reqs` rate)
- latency (`p95`, `p99`)
- error rate
- maximum VUs used (proxy for concurrent clients supported)
- container CPU and memory (sampled via `docker stats`)

Saturation methodology:

1. Increase target RPS in fixed steps.
2. Mark a step unstable when any of these occur:
   - error rate `>= 2%`
   - `p95 > 500ms`
   - `p99 > 1000ms`
3. Detect a latency knee when `p95` increases by more than `50%` versus previous step.
4. Report the first unstable or knee step as saturation.

Outputs:

- detailed summary: `loadtest/capacity/capacity-summary.json`
- methodology doc: `loadtest/capacity/methodology.md`
- per-step k6 summaries: `loadtest/capacity/step-<RPS>.json`
- per-step container stats: `loadtest/capacity/step-<RPS>-docker-stats.csv`

Capacity metrics in summary JSON:

- `maximum_concurrent_clients_supported`
- `max_qps_before_saturation`
- `cpu_usage_at_saturation_pct`
- `memory_usage_at_saturation_pct`
- `saturation_step` (includes p95/p99/error rate and knee flag)

Resume formulas:

- Hit stable capacity (QPS): max achieved `qps` across stable steps
- Latency knee point: first step where `(current_p95 - previous_p95) / previous_p95 > 0.5`
- Throughput before degrade: same as max stable QPS
- Concurrency supported: max `max_vus` across stable steps

## API Test Plan

Use this quick checklist before calling it production-ready:

- CRUD path: create/list/get/update/delete for both category and product.
- Validation: invalid UUID, negative price/stock, invalid currency, invalid pagination.
- Filters: price range, category filter, nameSearch (`ILIKE`).
- Sorting and pagination: all sort fields + ASC/DESC + limit/offset edge values.
- Relationship queries: `category { products }` and `product { category }`.
- Error contract: verify `extensions.code` values (`INVALID_ARGUMENT`, `NOT_FOUND`, etc.).
- Runtime: `/healthz`, DB reconnect after restart, auth behavior if enabled.

## Smoke Test Script

An executable smoke test is included at `scripts/smoke_test.sh`.

It runs:

- `createCategory`
- `createProduct`
- filtered `products` query
- `category(id) { products(...) }` nested query
- invalid UUID validation query
- `deleteProduct`

Run it:

```bash
make smoke-test
```

Or with a custom endpoint:

```bash
API_URL=http://localhost:8080/query bash scripts/smoke_test.sh
```

If Basic Auth is enabled:

```bash
BASIC_AUTH_USERNAME=admin BASIC_AUTH_PASSWORD=change-me make smoke-test
```

## Notes

- Product/category IDs are UUIDs.
- DB indexes created for `products.name` and `products.category_id`.
- Dataloader batches `Product.category` lookups per request.
