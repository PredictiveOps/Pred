# Event Processing Service

Consumes raw events from Kafka, persists them to PostgreSQL, and exposes a lightweight HTTP adapter for API-gateway compatibility. The service is multi-tenant: every event is stored with a `tenant_id` so data remains tenant-scoped.

## What it does

- **Consumes** event messages from a configured Kafka topic using a consumer group, which allows horizontal scaling.
- **Persists** each incoming event into PostgreSQL together with its tenant context and raw payload.
- **Exposes** a small HTTP adapter for health checks and gateway-driven event submission.
- **Prepares** a shared processing layer for future downstream event logic.

## Responsibilities

- **Kafka ingestion** — reads raw events from the configured Kafka topic.
- **Persistence** — stores every event in the database.
- **HTTP adapter** — supports API-gateway integration without bypassing the normal event flow.
- **Processing stub** — downstream processing logic is currently minimal and can be extended later.

## Endpoints

- `GET /health` — health check.
- `POST /events` — accepts an event payload and stores it through the same internal service used by Kafka ingestion.

## Database

Events are persisted in PostgreSQL. Schema is managed by GORM via `AutoMigrate` on startup, so no separate migration tool is needed. Models live in the `db` package.

## Multi-tenancy

Every event is scoped to a tenant. Tenant context is carried in the Kafka message and used for data isolation in the database.

## Configuration

| Variable         | Default                                       | Description                                                                 |
| ---------------- | --------------------------------------------- | --------------------------------------------------------------------------- |
| `KAFKA_BROKERS`  | `localhost:9092`                            | Comma-separated list of Kafka bootstrap brokers                             |
| `KAFKA_TOPIC`    | `events`                                    | Topic the consumer subscribes to                                             |
| `KAFKA_GROUP_ID` | `event-processing-service`                  | Consumer group ID — instances sharing this ID split partitions between them   |
| `DATABASE_URL`   | `postgres://localhost:5432/events`          | PostgreSQL connection string                                                 |

## Running

Start the shared infrastructure from the repository root first:

```sh
docker compose up -d
```

Then start the service:

```sh
cd event-processing-service
cp .env.example .env
# fill in values
go run .
```

The service starts both the Kafka consumer loop and the HTTP server on `:8080`.

## Testing

Integration tests need their own Postgres (separate from the dev one) and skip when `TEST_DATABASE_URL` is unset. Use the Makefile targets — they bring up `../docker-compose.test.yml` at the repo root (shared test Postgres on host port `5434`, db `events_test`) and inject the env var:

```sh
make test       # starts test Postgres, waits for healthy, runs `go test ./...`
make test-down  # tears down the test container and volume
```

The test compose runs alongside the dev `docker-compose.yml` without conflict.

## Notes

- Schema is managed through GORM `AutoMigrate` on startup.
- Models and query logic live in the `db` package.
- The Kafka consumer and HTTP adapter both call the same internal service layer, so the DB write path stays consistent.
