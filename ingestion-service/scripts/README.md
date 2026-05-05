# Integration Test Scripts

Test scripts for the ingestion service pipeline (device registration → MQTT → signature verification → Kafka).

## Scripts

### `run_integration_test.sh`
Complete end-to-end integration test with 14 assertions covering:
- Infrastructure validation
- Device creation and registration
- ECDSA signature generation and verification
- MQTT publishing
- Kafka message consumption
- Replay protection testing

**Usage:**
```bash
# From ingestion-service directory
chmod +x scripts/run_integration_test.sh
./scripts/run_integration_test.sh
```

**Expected output:**
```
✓ Docker Compose services running
✓ Ingestion service health check
✓ Generate ECDSA keypair
...
✓ Replay protection (nonce rejection)

Test Results:
Passed: 14
Failed: 0
```

### `sign_mqtt_payload.py`
Python utility to generate ECDSA-signed MQTT telemetry payloads.

**Dependencies:**
```bash
pip install ecdsa
```

**Usage:**
```bash
# Generate a signed payload
python3 scripts/sign_mqtt_payload.py /path/to/device-private.pem

# Output is JSON suitable for MQTT publishing:
# {
#   "timestamp": 1714756800,
#   "nonce": "n-1714756800000",
#   "data": { ... },
#   "signature": "BASE64_ECDSA_SIGNATURE"
# }
```

**Parameters (customizable in code):**
- `mode`: Device mode (default: "normal")
- `v_rms`: RMS voltage (default: 1.23)
- `temp_c`: Temperature in Celsius (default: 72.4)
- `peak_hz_1`, `peak_hz_2`, `peak_hz_3`: Peak frequencies (default: 50, 100, 150)
- `status`: Status (default: "ok")

Edit the script to change default sensor values.

## Prerequisites

- Docker + docker-compose (all services running)
- OpenSSL (for keypair generation)
- Python 3.7+ with `ecdsa` package
- `jq` (for JSON parsing in test script)
- `curl` (for HTTP requests)

## Quick Start

1. **Setup**
   ```bash
   cd ingestion-service
   chmod +x scripts/*.sh
   ```

2. **Run full test**
   ```bash
   ./scripts/run_integration_test.sh
   ```

3. **Manual testing** (if needed)
   ```bash
   # Generate keypair
   openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out /tmp/device-private.pem
   
   # Generate signed payload
   python3 scripts/sign_mqtt_payload.py /tmp/device-private.pem
   
   # Manually publish via mosquitto_pub
   docker compose exec mosquitto mosquitto_pub \
     -h localhost -p 8883 \
     --cafile /etc/mosquitto/config/certs/ca.crt \
     -u pred-device -P dev-device-password \
     -i 1 -t 'devices/1/data' \
     -m '{"timestamp":...,"nonce":"...","data":{...},"signature":"..."}'
   ```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `No such file or directory` | Make sure scripts are executable: `chmod +x scripts/*.sh` |
| `ModuleNotFoundError: ecdsa` | Install: `pip install ecdsa` |
| `docker-compose: command not found` | Use `docker compose` (newer versions) instead |
| `Signature verification failed` | Check field order in JSON (must be alphabetical) |
| `Nonce already used` | Wait 60+ seconds for Redis TTL to expire |

## Integration Test Flow

```
1. Infrastructure check (Docker, services, health)
2. Keypair generation (ECDSA P-256)
3. Device creation (HTTP POST)
4. Public key registration (MQTT)
5. Verify storage in database
6. Generate signed telemetry
7. Publish to MQTT broker
8. Verify in Kafka topic
9. Validate payload structure
10. Test replay protection
```

See `INTEGRATION_TEST_RUNBOOK.md` for detailed step-by-step instructions.
