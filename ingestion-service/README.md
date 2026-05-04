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
| `PORT`            | `2500`                         | Port the HTTP API listens on                                                |
| `DATABASE_URL`    | required                       | PostgreSQL connection string                                                |
| `KAFKA_BROKERS`   | `localhost:9092`               | Comma-separated list of Kafka bootstrap brokers                             |
| `KAFKA_TOPIC`     | required                       | Kafka topic used for published device events                                |
| `MQTT_BROKER`     | `ssl://localhost:8883`         | MQTT broker URL                                                             |
| `MQTT_CLIENT_ID`  | required                       | MQTT client ID                                                              |
| `MQTT_TOPIC`      | required                       | MQTT topic subscribed to for device data                                    |
| `MQTT_USERNAME`   | optional                       | MQTT username                                                               |
| `MQTT_PASSWORD`   | optional                       | MQTT password                                                               |
| `MQTT_CA_CERT`    | optional                       | CA certificate path for private/self-signed MQTTS broker certificates       |
| `MQTT_DEVICE_REGISTRATION_TOPIC` | required | MQTT topic for device public key registration (e.g., `devices/+/registration`) |
| `MQTT_DEVICE_REGISTRATION_RESPONSE_TOPIC` | required | Template for MQTT registration response topic (e.g., `devices/%d/registration/response`) |
| `REDIS_ADDR`      | `localhost:6379`               | Redis address used for device public key cache and nonce replay protection  |
| `REDIS_PASSWORD`  | empty                          | Redis password                                                              |
| `REDIS_DB`        | `0`                            | Redis DB index                                                              |
| `REDIS_PUBKEY_TTL`| `30m`                          | TTL for `device_pubkey:<device_id>` cache entries                           |
| `REDIS_NONCE_TTL` | `60s`                          | TTL for `nonce:<device_id>:<nonce>` replay-protection entries               |

## Running

```sh
cp .env.example .env
# fill in values
go run .
```

## HTTP API: Device registration

Register a device (creates DB entry). Endpoint:

- `POST /devices/register` — body: `{ "device_id": <uint>, "tenant_id": <uint> }`

Example:

```sh
curl -s -X POST http://localhost:2500/devices/register \
	-H 'Content-Type: application/json' \
	-d '{"device_id":1,"tenant_id":1}' | jq .
```

On success you will receive `{"registration_status":"ok"}` (HTTP 201).

If you see `{"error":"failed to register device"}` the service logged the underlying DB error — check the service logs for details (timestamp mismatch or schema issues are common when models change).

## MQTT Device Data Payload Format

Devices send signed telemetry via MQTT to `devices/{deviceID}/data`. The payload envelope must contain:

```json
{
	"timestamp": 1704067200,
	"nonce": "n-1",
	"data": {
		"mode": "normal",
		"v_rms": 1.23,
		"temp_c": 72.4,
		"peak_hz_1": 50,
		"peak_hz_2": 100,
		"peak_hz_3": 150,
		"status": "ok"
	},
	"signature": "BASE64_ENCODED_ECDSA_SIGNATURE"
}
```

**Important: JSON Field Order**

The signature is computed over the exact **byte sequence** of the `data` object, **not** a re-marshaled version. This means:

1. **Device** signs the byte representation of its `data` object using ECDSA + SHA256.
2. **Server** receives the envelope, extracts `data` as raw bytes, and verifies the signature against those exact bytes.
3. If field order differs between device and server JSON marshaling, verification **will fail**.

**Best Practice**: Use canonical JSON (alphabetically sorted keys) or document a fixed field order, so both sides match:

**Canonical order** for `data`:
```
mode, peak_hz_1, peak_hz_2, peak_hz_3, status, temp_c, v_rms
```

Example device pseudo-code:
```python
data = {
	"mode": "normal",
	"peak_hz_1": 50,
	"peak_hz_2": 100,
	"peak_hz_3": 150,
	"status": "ok",
	"temp_c": 72.4,
	"v_rms": 1.23
}
data_bytes = json.dumps(data, separators=(',', ':')).encode('utf-8')  # no spaces
signature = sign(sha256(data_bytes), private_key)
envelope = {
	"timestamp": int(time.time()),
	"nonce": "unique-id-per-message",
	"data": data,
	"signature": base64.encode(signature)
}
mqtt.publish(f"devices/{device_id}/data", json.dumps(envelope))
```

## Security Notes

**⚠️ IMPORTANT - MQTT TLS Configuration:**
The current code uses `InsecureSkipVerify: true` in `services/MQTT.service.go` which disables TLS certificate validation. This is acceptable for local development but **must be disabled in production**.

Before deploying to production:
1. Generate proper TLS certificates with SANs covering your broker's hostname(s)
2. Remove or set `InsecureSkipVerify: false` in `services/MQTT.service.go`
3. Configure the CA certificate path via `MQTT_CA_CERT` environment variable

## Tests

Integration tests need their own Postgres (separate from the dev one) and skip when `TEST_DATABASE_URL` is unset. Use the Makefile targets — they bring up `../docker-compose.test.yml` at the repo root and inject the env var:

```sh
make test       # starts test Postgres, waits for healthy, runs `go test ./...`
make test-down  # tears down the test container and volume
```

The test compose runs alongside the dev `docker-compose.yml` without conflict.
