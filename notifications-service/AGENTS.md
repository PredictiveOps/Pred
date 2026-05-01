# Notifications Service — Agent Guidelines

## Testing

Always run `make test` (not bare `go test`) to execute integration tests — `TEST_DATABASE_URL` is only set by the Makefile. To run a single test: `go test ./db -run TestX` after starting the test DB manually or via `make test`.

## Schema & models

GORM `AutoMigrate` runs on startup — the models in the `db/` package are the single source of truth for schema. Do not document tables, columns, or indexes anywhere else (README, comments, etc.).

## Multi-tenancy

Every DB query must be scoped to `tenant_id`. `tenant_id` flows from the Kafka `AlertEvent` message — thread it through to every `db.*` call. Never omit the tenant filter.

## Delivery fan-out

Push notifications fan out to device tokens, not users. Use `db.DeviceTokensForUsers` (tenant-scoped) to resolve `user_id`s → tokens; create one `delivery` row per token.

## Stubs

`sendPush` and `sendEmail` in `main.go` are intentional stubs — they log only. Do not wire real integrations unless explicitly asked.

## Formatting

Use standard Go formatting (`gofmt` / `go fmt`). No other linter is configured.
