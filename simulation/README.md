# Simulation Tools

This folder contains data simulators for MQTT and ML prediction replay.

## Raw Telemetry Engine

Run `raw_telemetry_engine.py` when you need switchable payload formats.

### Format toggle

Use `--format` to switch payload schema:

- `--format new` (default)
  - `device_name`
  - `timestamp`
  - `vibration_x`
  - `vibration_y`
  - `temp_motor`
  - `temp_atmospheric`

- `--format old`
  - `device_id`
  - `asset_id`
  - `timestamp`
  - `vibration_x` (array of 10 samples)
  - `vibration_y` (array of 10 samples)
  - `temperature_bearing`
  - `temperature_atmospheric`

Examples:

```bash
python3 simulation/raw_telemetry_engine.py --format new --rate 500 --workers 2 --duration 60
python3 simulation/raw_telemetry_engine.py --format old --rate 500 --workers 2 --duration 60 --asset bearing_motor_001
```

### Signature for new/old formats

Use `--signed` with a device private key PEM to generate the signature accepted by ingestion.

```bash
python3 simulation/raw_telemetry_engine.py --format new --device 1 --signed --private-key /absolute/path/device-private.pem --count 10 --duration 0
python3 simulation/raw_telemetry_engine.py --format old --device 1 --signed --private-key /absolute/path/device-private.pem --count 10 --duration 0
```

How signature is built:

- Telemetry object (new or old schema) is serialized as compact JSON.
- ECDSA-SHA256 signature is computed over the exact bytes of that `data` JSON.
- Final MQTT payload is wrapped as:

```json
{
  "timestamp": 1715000000,
  "nonce": "n-123",
  "data": { ... new-or-old-schema ... },
  "signature": "BASE64_ECDSA_SIGNATURE"
}
```

## Kafka forwarding compatibility (important)

The current `ingestion-service` expects signed MQTT envelopes on topic pattern:

- `devices/{numeric_device_id}/data`

Envelope shape expected by ingestion:

```json
{
  "timestamp": 1715000000,
  "nonce": "n-123",
  "data": {
    "mode": "normal",
    "v_rms": 1.18,
    "temp_c": 72.4,
    "peak_hz_1": 50,
    "peak_hz_2": 100,
    "peak_hz_3": 150,
    "status": "ok"
  },
  "signature": "BASE64_ECDSA_SIGNATURE"
}
```

Because of this, `raw_telemetry_engine.py` formats (`new` and `old`) are useful for raw MQTT load simulation, but are **not** accepted by ingestion for Kafka forwarding unless you add a mapper/signer or extend ingestion schema validation.

## Ingestion schema switch

`ingestion-service` now supports payload schema mode via `MQTT_PAYLOAD_FORMAT`:

- `auto` (default): tries ingestion schema, then new schema, then old schema
- `ingestion`: accept only the original `mode/v_rms/temp_c/peak_hz_*/status` schema
- `new`: accept only new schema (`device_name`, scalar vibration/temp)
- `old`: accept only old schema (legacy vibration arrays + temperatures)

Set it in compose env or shell before start:

```bash
export MQTT_PAYLOAD_FORMAT=auto
docker compose up -d ingestion-service
```
