# Event Processing Service

Consumes raw events from a Kafka topic and persists them to PostgreSQL for downstream processing, and exposes an HTTP API used by the web frontend. Designed as a multi-tenant service where each tenant's events are isolated.

## Responsibilities

- **Consume** — reads raw event messages from a configured Kafka topic using a consumer group, enabling horizontal scaling
- **Persist** — stores every event in the database with tenant context and the original payload
- **Process** — placeholder for downstream processing logic (currently a stub)
- **Serve** — exposes HTTP endpoints (powered by `gin`) consumed by the web frontend; runs alongside the Kafka consumer in the same process

## Database

Events are persisted in PostgreSQL. Schema is managed by GORM via `AutoMigrate` on startup — idempotent, no separate migration tool needed. Models live in the `db` package.

## Multi-tenancy

Every event is scoped to a tenant. Tenant context is carried in the Kafka message and used for data isolation in the database.

## Configuration

| Variable         | Default                                       | Description                                                                 |
| ---------------- | --------------------------------------------- | --------------------------------------------------------------------------- |
| `KAFKA_BROKERS`  | `localhost:9092`                              | Comma-separated list of Kafka bootstrap brokers                             |
| `KAFKA_TOPIC`    | `events`                                      | Topic the consumer subscribes to                                            |
| `KAFKA_GROUP_ID` | `event-processing-service`                    | Consumer group ID — instances sharing this ID split partitions between them |
| `DATABASE_URL`   | `postgres://localhost:5432/events`            | PostgreSQL connection string                                                |
| `HTTP_PORT`      | `8080`                                        | Port the HTTP API listens on                                                |

## Running

```sh
cp .env.example .env
# fill in values
go run .
```

## Tests

Integration tests need their own Postgres (separate from the dev one) and skip when `TEST_DATABASE_URL` is unset. Use the Makefile targets — they bring up `../docker-compose.test.yml` at the repo root (shared test Postgres on host port `5434`, db `events_test`) and inject the env var:

```sh
make test       # starts test Postgres, waits for healthy, runs `go test ./...`
make test-down  # tears down the test container and volume
```

The test compose runs alongside the dev `docker-compose.yml` without conflict.
