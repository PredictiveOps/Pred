# Ingestion Service

Accepts device telemetry over MQTT, persists devices in PostgreSQL, and publishes the ingested payload to Kafka for downstream processing. The service also exposes a small HTTP API for health checks and basic device lookup/registration.

## Responsibilities

- **Ingest** — subscribes to the configured MQTT topic and handles incoming device messages
- **Persist** — stores device records in PostgreSQL via GORM
- **Publish** — forwards MQTT messages to Kafka using a producer
- **Serve** — exposes HTTP endpoints for health checks and device CRUD reads

## Database

Devices are persisted in PostgreSQL. Schema is managed by GORM via `AutoMigrate` on startup — idempotent, no separate migration tool needed. Models and queries live in the `db` package.

## Configuration

| Variable          | Default                        | Description                                                                 |
| ----------------- | ------------------------------ | --------------------------------------------------------------------------- |
| `PORT`            | `8080`                         | Port the HTTP API listens on                                                |
| `DATABASE_URL`    | required                       | PostgreSQL connection string                                                |
| `KAFKA_BROKERS`   | `localhost:9092`               | Comma-separated list of Kafka bootstrap brokers                             |
| `KAFKA_TOPIC`     | required                       | Kafka topic used for published device events                                |
| `MQTT_BROKER`     | `tcp://localhost:1883`         | MQTT broker URL                                                             |
| `MQTT_CLIENT_ID`  | required                       | MQTT client ID                                                              |
| `MQTT_TOPIC`      | required                       | MQTT topic subscribed to for device data                                    |
| `MQTT_USERNAME`   | optional                       | MQTT username                                                               |
| `MQTT_PASSWORD`   | optional                       | MQTT password                                                               |

## Running

```sh
cp .env.example .env
# fill in values
go run .
```

## Tests

Integration tests need their own Postgres (separate from the dev one) and skip when `TEST_DATABASE_URL` is unset. Use the Makefile targets — they bring up `../docker-compose.test.yml` at the repo root and inject the env var:

```sh
make test       # starts test Postgres, waits for healthy, runs `go test ./...`
make test-down  # tears down the test container and volume
```

The test compose runs alongside the dev `docker-compose.yml` without conflict.
