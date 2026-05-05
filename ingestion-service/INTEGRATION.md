# Ingestion-Service Integration

This guide covers only the `ingestion-service` integration: how to send telemetry (MQTT/HTTP), the canonical payloads produced to Kafka, environment variables needed, and test commands.

## Quick dev setup
1. Start local infra (Kafka, optional mosquitto) if not already running:

```sh
docker-compose up -d
```

2. Run the `ingestion-service` locally:

```sh
cd ingestion-service
go run .
```

3. Confirm service health:

```sh
curl http://localhost:2500/health
```


## MQTT Device Registration Flow

Before devices can send telemetry, they must be registered:

1. **Create device record via HTTP**:
```sh
curl -X POST http://localhost:2500/devices/register \
  -H 'Content-Type: application/json' \
  -d '{"device_id": 1, "tenant_id": 1}'
```

2. **Register public key via MQTT**:
```sh
# Read public key and publish to registration topic
PUBLIC_KEY=$(cat /tmp/test-device-public.pem)
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-device -P dev-device-password \
  -i 1 \
  -t 'devices/1/registration' \
  -m "$(jq -nc --arg pk "$PUBLIC_KEY" '{public_key:$pk}')"
```

## Signed MQTT Telemetry Payload

Devices send cryptographically signed telemetry to `devices/{deviceID}/data`:

```json
{
  "timestamp": 1704067200,
  "nonce": "n-1234567890",
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

### Field Requirements
- `timestamp`: Unix timestamp (seconds)
- `nonce`: Unique per message (replay protection)
- `data`: Sensor readings object
- `signature`: ECDSA signature over exact bytes of `data`

## Test Commands

### Generate Test Keypair
```sh
# Private key
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out /tmp/device-private.pem

# Public key  
openssl pkey -in /tmp/device-private.pem -pubout -out /tmp/test-device-public.pem
```

### Sign and Publish Telemetry
```sh
# Generate signed payload
python3 scripts/sign_mqtt_payload.py /tmp/device-private.pem > /tmp/signed_payload.json

# Publish via MQTT
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-device -P dev-device-password \
  -i 1 \
  -t 'devices/1/data' \
  -f /tmp/signed_payload.json
```

### Verify Kafka Output
```sh
docker compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic sensor_data \
  --from-beginning \
  --max-messages 1
```

## Integration Notes

- **Kafka Topic**: `sensor_data` (not `events`)
- **Message Flow**: Device → MQTT (signed) → Verification → Kafka
- **No HTTP Ingestion**: Service only accepts telemetry via MQTT
- **Signature Verification**: All telemetry must be cryptographically signed
- **Replay Protection**: Nonces are tracked to prevent replay attacks
## Security Notes
- For production, require TLS for MQTT and authenticate devices with client certificates
- Secure Kafka with TLS and SASL; do not expose brokers directly to the public internet

**⚠️ IMPORTANT - TLS Certificate Verification:**
The current MQTT client configuration uses `InsecureSkipVerify: true` which disables TLS certificate validation. This is acceptable for local development but **must be disabled in production**.

Before deploying to production:
1. Generate proper TLS certificates with SANs covering your broker's hostname(s)
2. Set `InsecureSkipVerify: false` in `services/MQTT.service.go`
3. Configure the CA certificate path via `MQTT_CA_CERT` environment variable

## Troubleshooting
- If messages are not appearing on `sensor_data`, check:
  - Device is registered and has a valid public key
  - MQTT TLS certificates are configured correctly
  - Signature verification is working (check device private/public key pair)
  - Redis is running for nonce replay protection
