# Event Processing Service — Agent Guidelines

## Testing

Always run `make test` (not bare `go test`) to execute integration tests — `TEST_DATABASE_URL` is only set by the Makefile. To run a single test: `go test ./db -run TestX` after starting the test DB manually or via `make test`.

## Schema & models

GORM `AutoMigrate` runs on startup — the models in the `db/` package are the single source of truth for schema. Do not document tables, columns, or indexes anywhere else (README, comments, etc.).

## Multi-tenancy

Every DB query must be scoped to `tenant_id`. `tenant_id` flows from the Kafka raw event message — thread it through to every `db.*` call. Never omit the tenant filter.

## Processing logic

Downstream event processing logic is a stub. Do not implement real processing unless explicitly asked.

## Formatting

Use standard Go formatting (`gofmt` / `go fmt`). No other linter is configured.
