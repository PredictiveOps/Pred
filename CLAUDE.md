@AGENT_GUIDELINES.md

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository layout

Monorepo for a multi-tenant predictive maintenance system (university SE module project). Top-level services:

- `notifications-service/` — Go service that consumes alert events from Kafka and delivers email/push notifications.
- `event-processing-service/` — Go service that consumes the raw events Kafka topic and persists each event to its own Postgres database (downstream processing logic is a stub).
- `web-frontend/` — Next.js 16 + React 19 app (Tailwind v4, Biome, shadcn/radix). See `web-frontend/AGENTS.md` — this Next.js version has breaking changes vs. training-data Next.js; consult `node_modules/next/dist/docs/` before writing frontend code.
- `postgres/init/` — entrypoint script that creates multiple databases inside the shared Postgres container, driven by `POSTGRES_MULTIPLE_DATABASES` (comma-separated).
- `docker-compose.yml` — dev infra: Postgres 18 (host port **5433**) and Kafka (host port **9092**, KRaft mode, auto-create topics on).
- `docker-compose.test.yml` — single isolated test Postgres on host port **5434** hosting both `notifications_test` and `events_test` databases (created by the same `postgres/init/create-databases.sh` script); runs alongside the dev compose without conflict.

## Common commands

Notifications service (run from `notifications-service/`):

```sh
go run .                  # run the consumer (loads .env)
make test                 # brings up test Postgres (--wait), runs `go test ./...` with TEST_DATABASE_URL set
make test-down            # tears down the test container + volume
go test ./db -run TestX   # run a single test
```

Event processing service (run from `event-processing-service/`) — same command surface (`go run .`, `make test`, `make test-down`).

Integration tests skip when `TEST_DATABASE_URL` is unset — always go through `make test` rather than bare `go test` if you want DB tests to run.

Web frontend (run from `web-frontend/`):

```sh
npm run dev      # next dev
npm run build
npm run lint     # biome check .
npm run format   # biome check --write .
```

## Architecture notes

**Notifications service** (`notifications-service/main.go`):
- Single Kafka consumer loop reading `AlertEvent` JSON messages. Each event has a `tenant_id`, a `type` (`"email"` or `"push"`), an opaque `payload`, and a list of `Recipients`.
- Per message: insert one `notifications` row, then fan out into `deliveries` rows (one per recipient/device token), attempt delivery, and update each delivery's status (`pending` → `delivered`/`failed`).
- Push fan-out resolves `user_id`s to device tokens via `db.DeviceTokensForUsers` (tenant-scoped) — one delivery per token, not per user.
- `sendPush` / `sendEmail` are stubs that just log; real integrations (Resend mentioned for email) are not wired yet.
- Schema is managed by GORM `AutoMigrate` on startup — no separate migration tool. Models in the `db` package are the source of truth; do not document schema in service READMEs (per repo policy in root README).

**Multi-tenancy** is enforced at the data-access layer: tenant ID flows from the Kafka message into every DB query. New queries must take and filter by `tenant_id`.

**Horizontal scaling** is via Kafka consumer groups — multiple instances sharing `KAFKA_GROUP_ID` split partitions.

## Repo conventions (from root README)

- Every service must have a `.env.example` with all required + optional vars and their defaults.
- Every service `README.md` must cover "what it does" and "how to run it" — and must **not** document database internals (tables/columns/indexes); that lives in code (models/migrations).
