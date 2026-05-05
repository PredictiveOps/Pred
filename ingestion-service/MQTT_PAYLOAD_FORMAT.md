# MQTT Payload Format & Signature Verification

This document specifies the exact format of signed MQTT telemetry that `ingestion-service` expects from devices.

## Overview

Devices send **cryptographically signed** sensor telemetry via MQTT to `devices/{deviceID}/data` where:
- `{deviceID}` is a numeric device ID (e.g., `devices/1/data`).
- The entire payload is a JSON envelope containing `timestamp`, `nonce`, `data`, and `signature` fields.
- The `signature` is computed over the exact **byte sequence** of the `data` object using ECDSA + SHA256.

## Payload Structure

```json
{
  "timestamp": 1704067200,
  "nonce": "unique-per-message",
  "data": {
    "mode": "normal",
    "peak_hz_1": 50,
    "peak_hz_2": 100,
    "peak_hz_3": 150,
    "status": "ok",
    "temp_c": 72.4,
    "v_rms": 1.23
  },
  "signature": "BASE64_ENCODED_ECDSA_SIGNATURE"
}
```

### Field Descriptions

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `timestamp` | int64 | ✓ | Unix timestamp (seconds since epoch) |
| `nonce` | string | ✓ | Unique ID per message; reusing within 60s is rejected (replay protection) |
| `data` | object | ✓ | Sensor readings (see schema below) |
| `signature` | string | ✓ | Base64-encoded ECDSA ASN.1 signature over raw bytes of `data` |

### `data` Field Schema

```json
{
  "mode": "string",      // e.g., "normal", "diagnostic"
  "peak_hz_1": "int",    // Peak frequency 1 in Hz
  "peak_hz_2": "int",    // Peak frequency 2 in Hz
  "peak_hz_3": "int",    // Peak frequency 3 in Hz
  "status": "string",    // e.g., "ok", "warning", "error"
  "temp_c": "float64",   // Temperature in Celsius
  "v_rms": "float64"     // RMS voltage
}
```

## Critical: JSON Field Order

The signature is computed over the **exact bytes** of the `data` object, not a re-marshaled version. This means:

1. **Device** must serialize `data` deterministically (e.g., fields in a fixed order, no extra whitespace).
2. **Server** extracts the `data` bytes and verifies the signature against those exact bytes.
3. **If field order differs** between device and server, signature verification **will fail**.

### Canonical Field Order (Alphabetical)

To ensure compatibility, both device and server should use this canonical order when marshaling the `data` object:

```
mode, peak_hz_1, peak_hz_2, peak_hz_3, status, temp_c, v_rms
```


## Registration Flow

Before a device can send telemetry:

1. **HTTP: Create shell device record**
   ```bash
   curl -X POST http://localhost:2500/devices/register \
     -H 'Content-Type: application/json' \
     -d '{"device_id": 1, "tenant_id": 1}'
   ```

2. **MQTT: Register public key**
   - Device publishes to `devices/1/registration`:
     ```json
     {
       "public_key": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----"
     }
     ```
   - Server responds on `devices/1/registration/response`:
     ```json
     {
       "registration_status": "ok"
     }
     ```

3. **MQTT: Send signed telemetry**
   - Device now publishes to `devices/1/data` with signed envelope (as specified above).

## Testing

### Generate Dev Keypair (ECDSA P-256)

```bash
# Private key
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out device-private.pem

# Public key
openssl pkey -in device-private.pem -pubout -out device-public.pem
```

### Test MQTT Publish (No Signature, For Learning)

```bash
# Single pub to trigger receive (without valid signature, it will be rejected)
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-device -P dev-device-password \
  -i 1 \
  -t 'devices/1/data' \
  -m '{"timestamp":1704067200,"nonce":"n-1","data":{"mode":"normal","peak_hz_1":50,"peak_hz_2":100,"peak_hz_3":150,"status":"ok","temp_c":72.4,"v_rms":1.23},"signature":"INVALID"}'
```

### Consume Kafka to Verify

```bash
docker compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic sensor_data \
  --from-beginning \
  --max-messages 1
```

## Common Pitfalls

1. **JSON field order mismatch** → Signature verification fails.
2. **Including extra whitespace** in data JSON → Bytes don't match.
3. **Reusing a nonce** within the TTL (default 60s) → Replay rejection.
4. **Device not registered** → Public key not found, all telemetry rejected.
5. **Using numeric device ID string** (e.g., `devices/device-001/data`) → Topic parsing fails (expects `devices/1/data`).
