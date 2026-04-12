APP_NAME=product-catalog-api
DB_URL=postgres://$${DB_USER}:$${DB_PASSWORD}@$${DB_HOST}:$${DB_PORT}/$${DB_NAME}?sslmode=$${DB_SSLMODE}

.PHONY: run gqlgen test build docker-up docker-down migrate-up migrate-down smoke-test

run:
	go run ./cmd/server

gqlgen:
	go run github.com/99designs/gqlgen generate

test:
	go test ./...

build:
	go build -o bin/server ./cmd/server

smoke-test:
	bash scripts/smoke_test.sh

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

migrate-up:
	migrate -path ./migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(DB_URL)" down 1
