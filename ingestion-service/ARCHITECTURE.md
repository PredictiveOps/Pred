# Ingestion-Service Architecture

This document describes the architecture of the `ingestion-service` (device-facing entrypoint), its responsibilities, inputs/outputs, deployment surfaces, and integration considerations. It is scoped to the ingestion microservice only.

## Purpose and responsibilities
- Accept signed telemetry from devices via MQTT.
- Verify message authenticity using each device's registered public key.
- Protect against replay attacks by validating the per-message nonce.
- Validate and forward verified device data to Kafka for downstream consumers.

## Component overview
- MQTT broker client: subscribes/accepts device topics at `devices/{deviceID}/data` and receives signed telemetry envelopes.
- Signature verification layer: extracts the raw `data` bytes, hashes them with SHA256, and verifies the ECDSA signature using the device's registered public key.
- Replay-protection store: tracks previously used nonces per device to reject replayed messages.
- Payload validator: validates the verified payload shape and rejects malformed device data.
- Kafka producer: publishes verified device data for downstream processing.
- Health + metrics surface: exposes operational status and monitoring signals for orchestration and observability.

## Logical flow
1. Device publishes a signed telemetry envelope to the MQTT topic `devices/{deviceID}/data`.
2. Ingestion receives the raw payload and extracts `timestamp`, `nonce`, `data`, and `signature`.
3. The service verifies the ECDSA signature over the exact raw bytes of `data` using the device's registered public key.
4. The service checks the `nonce` to prevent replay attacks and validates the decoded `data` payload.
5. If all checks pass, the verified device data is forwarded to Kafka and the MQTT publish is acknowledged; otherwise the message is rejected and logged.

## MQTT Device Data Payload (device → ingestion)

Devices publish signed telemetry to `devices/{deviceID}/data`. The payload envelope must contain an ECDSA signature over the exact bytes of the `data` object.

```json
{
  "timestamp": 1704067200,
  "nonce": "unique-nonce-per-message",
  "data": {
    "mode": "normal",
    "peak_hz_1": 50,
    "peak_hz_2": 100,
    "peak_hz_3": 150,
    "status": "ok",
    "temp_c": 72.4,
    "v_rms": 1.23
  },
  "signature": "BASE64_ENCODED_ECDSA_SHA256_SIGNATURE"
}
```

**Signature Verification Logic:**
1. Extract `data` as raw bytes from the JSON payload (not re-marshaled).
2. Compute SHA256 hash of those bytes.
3. Verify ECDSA signature against the hash using the device's registered public key.
4. Check `nonce` for replay attacks (Redis-backed list of used nonces per device).
5. If all checks pass, unmarshal `data` and forward to Kafka.

**Important**: JSON field order in `data` must be deterministic (canonically ordered). If device and server marshal JSON differently, signature verification will fail.

## Kafka Output Payload (ingestion → Kafka)

The ingestion service publishes the sensor data to Kafka with device metadata:

```json
{
  "device_id": 1,
  "timestamp": 1704067200,
  "mode": "normal",
  "v_rms": 1.23,
  "temp_c": 72.4,
  "peak_hz_1": 50,
  "peak_hz_2": 100,
  "peak_hz_3": 150,
  "status": "ok"
}
```

- `message_id` should be a stable id for idempotency/deduplication downstream.
- `timestamp` is device-supplied event time; `received_at` is ingestion time.

## Topics, keys and partitioning
- Topic: `events` (configurable via `KAFKA_TOPIC_EVENTS`).
- Producer key: prefer `tenant_id` for co-location by tenant, or `device_id` for strict per-device ordering. Document your choice in deployment config.

## Environment and configuration (key vars)
- `KAFKA_BROKERS` — e.g., `localhost:9092`
- `KAFKA_TOPIC_EVENTS` — default `events`
- `KAFKA_GROUP_ID` — used when ingestion contains any consumer parts (optional)
- `MQTT_BROKER_URL` — URL for MQTT broker (e.g., `tcp://mosquitto:1883`)
- `HTTP_BIND_ADDR` — HTTP listen address (e.g., `:2500`)
- `LOG_LEVEL` — logging verbosity
- `DATABASE_URL` — only if the ingestion service needs a local DB for dedupe/offsets (not required in current implementation)

## Security
- Require `tenant_id` in every message. If you support multi-tenant devices, authenticate devices and map credentials to tenant.
- For production: enable TLS on MQTT and HTTPS for HTTP endpoints; configure client certs or token-based auth.
- Secure Kafka with TLS and SASL in production.

**⚠️ IMPORTANT - MQTT TLS Configuration:**
The current code uses `InsecureSkipVerify: true` in the TLS config to handle localhost development scenarios where the broker certificate may not match the connection IP. **This disables TLS certificate validation and is insecure for production.**

Before deploying to production:
1. Generate proper TLS certificates with SANs covering your broker's hostname(s)
2. Remove or set `InsecureSkipVerify: false` in `services/MQTT.service.go`
3. Configure all clients to verify the broker's certificate against a trusted CA

## Observability & health
- Expose `/health` for liveness and readiness.
- Expose `/metrics` for Prometheus scraping (request rate, success/failure counts, Kafka publish latency, etc.).
- Log structured JSON including `tenant_id`, `device_id`, `message_id` for traceability.

## Operational considerations
- Backpressure: if Kafka is unavailable, the service should buffer to local disk (or return 503 for HTTP). Avoid unbounded memory buffering.
- Retries: implement limited retries with exponential backoff for Kafka publish failures; consider a dead-letter topic for messages failing schema validation or persistent failures.
- Idempotency: downstream consumers will use `message_id` for deduplication. Ensure the ingestion service generates stable IDs if retries occur.

## Testing & local run
- Use the repo `docker-compose.yml` to bring up a local MQTT broker (mosquitto) and Kafka/Postgres as needed.
- Quick test commands (examples):

```sh
# Generate test keypair
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out /tmp/device-private.pem
openssl pkey -in /tmp/device-private.pem -pubout -out /tmp/device-public.pem

# Register device
curl -X POST http://localhost:2500/devices/register \
  -H 'Content-Type: application/json' \
  -d '{"device_id": 1, "tenant_id": 1}'

# Generate signed telemetry payload
python3 scripts/sign_mqtt_payload.py /tmp/device-private.pem > /tmp/signed-payload.json

# Publish signed telemetry via MQTT
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-device -P dev-device-password \
  -i 1 \
  -t 'devices/1/data' \
  -f /tmp/signed-payload.json
```

## Integrations (downstream)
- `event-processing-service` consumes `sensor_data` topic — coordinate the `KAFKA_TOPIC` name and partitioning key.
- Alerting/notification flows depend on downstream processors; ingestion should not emit alerts directly.

---
For step-by-step integration commands and payload examples, see the ingestion-focused integration guide: INTEGRATION.md.
