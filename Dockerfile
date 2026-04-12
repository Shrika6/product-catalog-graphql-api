FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/server ./cmd/server

FROM alpine:3.21
WORKDIR /app
RUN adduser -D -g '' appuser

COPY --from=builder /bin/server /app/server
COPY migrations /app/migrations

USER appuser
EXPOSE 8080

CMD ["/app/server"]
