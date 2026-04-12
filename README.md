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
- Dataloader for category lookups (`Product.category`) to avoid N+1
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
│   └── service/
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
- `nameSearch` (`ILIKE`)

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
Playground (non-production): `http://localhost:8080/`

## Docker Run

```bash
docker compose up --build
```

This starts:

- `app` on `:8080`
- `postgres` on `:5432`

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

## Basic Auth (Optional)

Set both env vars to enable auth on `/query`:

```env
BASIC_AUTH_USERNAME=admin
BASIC_AUTH_PASSWORD=change-me
```

## Development Commands

```bash
make gqlgen   # regenerate gqlgen files after schema changes
make test     # run unit tests
make build    # build binary
make smoke-test # run API smoke test script
```

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
