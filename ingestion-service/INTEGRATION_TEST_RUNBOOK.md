# Ingestion Service Integration Testing Runbook

Complete step-by-step guide for testing the full ingestion pipeline: device creation → MQTT registration → signed telemetry → Kafka validation.

**Duration**: ~10-15 minutes for full flow.  
**Target Audience**: Backend engineers, QA, DevOps.  
**Prerequisite Knowledge**: MQTT, Kafka, ECDSA signatures, JSON serialization.

## Quick Start

**Run the automated end-to-end test:**
```bash
cd ingestion-service
./scripts/run_integration_test.sh
```

For manual step-by-step testing, follow the sections below.

---

## Part 1: Environment Validation & Prerequisites

### 1.1: Verify Infrastructure Is Running

```bash
# Check all required services
docker compose ps

# Expected output should show:
# - postgres (running on port 5433)
# - kafka (running on port 9092)
# - mosquitto (running on port 8883 with TLS)
# - redis (running on port 6379)
# - ingestion-service (running on port 2500)
```

**Expected Output:**
```
CONTAINER ID   IMAGE                COMMAND                  CREATED      STATUS      PORTS
...            postgres:18           "docker-entrypoint..."   ...          Up ...      0.0.0.0:5433->5432/tcp
...            confluentinc/cp-kafka "...kafka-server..."    ...          Up ...      0.0.0.0:9092->9092/tcp
...            eclipse-mosquitto    "mosquitto -c /moss..."  ...          Up ...      0.0.0.0:8883->8883/tcp
...            redis:7              "redis-server"           ...          Up ...      0.0.0.0:6379->6379/tcp
```

**If services are not running:**
```bash
# Start all services from repo root
cd /path/to/Pred
docker compose up -d

# Wait for health
docker compose ps
```

### 1.2: Verify Ingestion Service Is Healthy

```bash
# Health check endpoint
curl -s http://localhost:2500/health | jq .

# Expected output:
# { "status": "ok" }
```

**If the service is not running:**
```bash
# From ingestion-service directory
cd ingestion-service
cp .env.example .env
# Edit .env to set DATABASE_URL, KAFKA_BROKERS, MQTT_BROKER, etc.
go run .  # runs on :2500
```

### 1.3: Verify Mosquitto TLS Certificates Exist

```bash
# Check certificate files
ls -la mosquitto/certs/

# Expected files:
# - ca.crt (CA certificate)
# - server.crt (server certificate)
# - server.key (server private key)
```

**If certificates are missing:**
```bash
cd mosquitto
./entrypoint.sh  # generates certs
cd ..
```

### 1.4: Verify Database and Tables

```bash
# Connect to test postgres and list tables
docker compose exec postgres psql \
  -U postgres \
  -d postgres \
  -c "SELECT datname FROM pg_database WHERE datname LIKE 'events%' OR datname LIKE 'notifications%';"

# Expected: Two databases should exist
```

---

## Part 2: Generate Test ECDSA Keypair

### 2.1: Create Device Keypair (If Not Already Exists)

```bash
# Generate ECDSA P-256 private key
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 \
  -out /tmp/device-private.pem

# Extract public key
openssl pkey -in /tmp/device-private.pem -pubout -out /tmp/device-public.pem

# Display the public key (you'll need this for registration)
cat /tmp/device-public.pem
```

**Expected Output:**
```
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
-----END PUBLIC KEY-----
```

### 2.2: Use the Python Signer Script

The signing script is included in the project:

```bash
# Generate signed payload using the project script
python3 scripts/sign_mqtt_payload.py /tmp/device-private.pem
```

See [scripts/sign_mqtt_payload.py](scripts/sign_mqtt_payload.py) for implementation details.

---

## Part 3: Device Creation & Registration

### 3.1: Create a Device Via HTTP API

```bash
# Create a new device associated with tenant_id=1
DEVICE_RESPONSE=$(curl -s -X POST http://localhost:2500/devices/register \
  -H 'Content-Type: application/json' \
  -d '{
    "device_id": 1,
    "tenant_id": 1
  }')

echo "Device creation response:"
echo "$DEVICE_RESPONSE" | jq .

# Expected output:
# {
#   "registration_status": "ok"
# }
```

**If creation fails, check:**
- Database connectivity: `docker compose logs postgres`
- HTTP service logs: `docker compose logs ingestion-service`

### 3.2: Verify Device In Database

```bash
# Query the device
docker compose exec postgres psql \
  -U postgres \
  -d ingestion \
  -c "SELECT device_id, tenant_id, public_key, is_active FROM devices LIMIT 5;"

# Expected: device_id=1, tenant_id=1, public_key=NULL, is_active=false

# Note: Devices are created inactive and become active only after
# successful MQTT public key registration
```

### 3.3: Register Device Public Key Via MQTT

Device publishes its public key to `devices/1/registration`:

Important: run only the command below. Do not paste it after another `docker compose exec` line or the `-P` flag will be merged into the previous token and `mosquitto_pub` will fail.

```bash
# Read the public key into a variable
PUBLIC_KEY=$(cat /tmp/device-public.pem)

# Publish registration message
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-device \
  -P dev-device-password \
  -i 1 \
  -t 'devices/1/registration' \
  -m "$(jq -nc --arg pk "$PUBLIC_KEY" '{public_key:$pk}')"
```

**Expected**: No output means the message was published successfully.

### 3.4: Monitor Ingestion Service Log for Registration Response

<!-- In another terminal:
```bash
# Watch logs in real-time
docker compose logs -f ingestion-service | grep -i "registration\|device"
```

**Expected log output:**
```
[INFO] Received registration message for device_id=1
[INFO] Updated device public key in database
[INFO] Published registration response to devices/1/registration/response
``` -->

### 3.5: Verify Public Key Was Stored

```bash
# Check if public key is now in the database
docker compose exec postgres psql \
  -U postgres \
  -d ingestion \
  -c "SELECT device_id, public_key FROM devices WHERE device_id = 1;"

# Expected: public_key should now be populated (BEGIN PUBLIC KEY...)
```

### 3.6: Monitor Redis Cache

```bash
# Check if device state is cached in Redis
docker compose exec redis redis-cli \
  GET "device_pubkey:1"

# Expected: The base64-encoded public key should appear
# (or the full PEM if cached as string)
```

---

## Part 4: Send Signed Telemetry Payload

### 4.1: Generate a Signed Telemetry Payload

```bash
# Generate signed payload using project script
SIGNED_PAYLOAD=$(python3 scripts/sign_mqtt_payload.py /tmp/device-private.pem)

echo "Generated payload:"
echo "$SIGNED_PAYLOAD" | jq .

# Expected output:
# {
#   "timestamp": 1714756800,
#   "nonce": "n-1714756800000",
#   "data": {
#     "mode": "normal",
#     "peak_hz_1": 50,
#     "peak_hz_2": 100,
#     "peak_hz_3": 150,
#     "status": "ok",
#     "temp_c": 72.4,
#     "v_rms": 1.23
#   },
#   "signature": "MEYCIQDx+..."
# }
```

### 4.2: Publish Signed Telemetry to MQTT

```bash
# Publish the signed payload to devices/1/data
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /etc/mosquitto/config/certs/ca.crt \
  -u pred-device \
  -P dev-device-password \
  -i 1 \
  -t 'devices/1/data' \
  -m "$SIGNED_PAYLOAD"
```

**Expected**: No output = success. Message forwarded to ingestion service.

### 4.3: Monitor Ingestion Service For Verification

In the logs terminal:
```bash
# Watch for signature verification logs
docker compose logs -f ingestion-service | grep -i "verify\|signature\|kafka"
```

**Expected output:**
```
[INFO] verified signed payload for device_id=1 payload_bytes=17
[INFO] Published to Kafka topic=sensor_data key=1 partition=0
```

---

## Part 5: Verify Message Reached Kafka

### 5.1: Consume From Kafka Topic

```bash
# Consume the sensor_data topic
docker compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic sensor_data \
  --from-beginning \
  --max-messages 1
```

**Expected output:**
```json
{"device_id":1,"timestamp":1714756800,"mode":"normal","v_rms":1.23,"temp_c":72.4,"peak_hz_1":50,"peak_hz_2":100,"peak_hz_3":150,"status":"ok"}
```

### 5.2: Verify Payload Structure

```bash
# Consume and parse with jq for clarity
docker compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic sensor_data \
  --from-beginning \
  --max-messages 1 | jq .

# Expected:
# {
#   "device_id": 1,
#   "timestamp": 1714756800,
#   "mode": "normal",
#   "v_rms": 1.23,
#   "temp_c": 72.4,
#   "peak_hz_1": 50,
#   "peak_hz_2": 100,
#   "peak_hz_3": 150,
#   "status": "ok"
# }
```

### 5.3: Verify Database Record

```bash
# Check that the message was stored
docker compose exec postgres psql \
  -U postgres \
  -d events_dev \
  -c "SELECT device_id, timestamp, mode, status FROM events LIMIT 5;"

# Expected: Record with device_id=1 matching the published data
```

---

## Part 6: Test Replay Protection (Nonce Validation)

### 6.1: Attempt to Replay the Same Message

```bash
# Publish the SAME payload again (same nonce)
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /etc/mosquitto/config/certs/ca.crt \
  -u pred-device \
  -P dev-device-password \
  -i 1 \
  -t 'devices/1/data' \
  -m "$SIGNED_PAYLOAD"

# Wait 1 second then check logs
sleep 1
docker compose logs -f ingestion-service | grep -i "nonce\|replay" | tail -5
```

**Expected output:**
```
[WARN] Nonce already used for device_id=1 nonce=n-1714756800000
[ERROR] Message rejected: replay attack detected
```

### 6.2: Wait for Nonce TTL to Expire

```bash
# Nonce TTL is 60s by default. Wait and retry.
echo "Waiting 65 seconds for nonce to expire..."
sleep 65

# Now publish the same payload again
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /etc/mosquitto/config/certs/ca.crt \
  -u pred-device \
  -P dev-device-password \
  -i 1 \
  -t 'devices/1/data' \
  -m "$SIGNED_PAYLOAD"

# Check logs for successful processing
docker compose logs ingestion-service | grep -i "verify\|kafka" | tail -3
```

**Expected**: Second message should be accepted (nonce expired).

---

## Part 7: Test Invalid Signature Rejection

### 7.1: Publish With Invalid Signature

```bash
# Create payload with corrupted signature
INVALID_PAYLOAD=$(cat << 'EOF'
{
  "timestamp": 1714756800,
  "nonce": "n-invalid-1",
  "data": {
    "mode": "normal",
    "peak_hz_1": 50,
    "peak_hz_2": 100,
    "peak_hz_3": 150,
    "status": "ok",
    "temp_c": 72.4,
    "v_rms": 1.23
  },
  "signature": "INVALIDSIGNATUREBASE64"
}
EOF
)

docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /etc/mosquitto/config/certs/ca.crt \
  -u pred-device \
  -P dev-device-password \
  -i 1 \
  -t 'devices/1/data' \
  -m "$INVALID_PAYLOAD"

# Check logs
docker compose logs ingestion-service | grep -i "signature\|invalid" | tail -3
```

**Expected output:**
```
[WARN] Signature verification failed for device_id=1
[ERROR] Message rejected: invalid signature
```

---

## Part 8: Full End-to-End Integration Test

### 8.1: Run the Automated Integration Test

The integration test script is included in the project at `scripts/run_integration_test.sh`.

```bash
# From ingestion-service directory, run the automated test
cd ingestion-service
./scripts/run_integration_test.sh
```

**Expected output:**
```
==========================================
Ingestion Service Integration Test
==========================================

✓ Docker Compose services running
✓ Ingestion service health check
✓ Generate ECDSA keypair
✓ Extract public key
✓ Create device (HTTP POST)
✓ Publish registration message (MQTT)
✓ Device public key stored in database
✓ Generate signed payload
✓ Validate JSON signature format
✓ Publish signed telemetry (MQTT)
✓ Consume message from Kafka
✓ Kafka message contains device_id
✓ Kafka payload has correct sensor data
✓ Replay protection (nonce rejection)

==========================================
Test Results:
Passed: 14
Failed: 0
==========================================
```

For implementation details, see [scripts/run_integration_test.sh](scripts/run_integration_test.sh).

---

## Part 9: Cleanup and Reset State

### 9.1: Clear Test Data

```bash
# Stop all services
docker compose down

# Remove test containers and volumes
docker compose down -v

# Restart fresh
docker compose up -d

# Wait for services to be healthy
sleep 10
docker compose ps
```

### 9.2: Restart Ingestion Service (If Needed)

```bash
# Kill and restart
docker compose restart ingestion-service

# Verify health
curl -s http://localhost:2500/health | jq .
```

### 9.3: Clean Redis Cache

```bash
# Clear all Redis keys
docker compose exec redis redis-cli FLUSHALL

# Verify
docker compose exec redis redis-cli DBSIZE
```

---

## Troubleshooting Guide

### Issue: "Connection refused" on MQTT publish

**Cause**: Mosquitto not running or TLS misconfigured.

**Fix**:
```bash
docker compose up -d mosquitto
docker compose logs mosquitto | head -20
```

### Issue: "Signature verification failed"

**Possible causes**:
1. Field order mismatch in `data` object
2. Extra whitespace in JSON serialization
3. Device public key not stored correctly

**Debug**:
```bash
# Check stored public key
docker compose exec postgres psql -U postgres -d events_dev \
  -c "SELECT public_key FROM devices WHERE device_id = 1;"

# Verify signature generation
python3 scripts/sign_mqtt_payload.py /tmp/device-private.pem | jq .data
```

### Issue: "Message not appearing in Kafka"

**Possible causes**:
1. Signature verification failed (check logs)
2. Nonce already used (wait 60+ seconds)
3. Device not registered (no public key)

**Debug**:
```bash
# Check ingestion service logs
docker compose logs ingestion-service | grep -i error

# Check Kafka connectivity
docker compose exec kafka kafka-broker-api-versions --bootstrap-server localhost:9092

# Verify Kafka topic exists
docker compose exec kafka kafka-topics --bootstrap-server localhost:9092 --list
```

### Issue: "Nonce validation failed"

**Cause**: Reusing the same nonce within 60 seconds.

**Fix**: Generate new nonce for each message (script does this automatically with timestamp).

---

## Key Metrics to Monitor

### Performance Expectations

| Step | Expected Duration | Notes |
|------|-------------------|-------|
| Device creation (HTTP) | <50ms | Database insert |
| Public key registration (MQTT) | <100ms | MQTT roundtrip + DB update |
| Signature verification | <20ms | Crypto operations on single core |
| Kafka publish | <50ms | Network roundtrip |
| **Total e2e (registration + 1 telemetry)** | **<500ms** | End-to-end latency |

### Redis Cache Hit Rate

```bash
# Monitor cache operations
docker compose exec redis redis-cli INFO stats | grep hit_rate
```

Expected: >80% hit rate for repeated device queries.

---

## Conclusion

The integration test validates:
- ✅ Device CRUD operations
- ✅ MQTT public key registration with TLS
- ✅ ECDSA signature verification with exact byte preservation
- ✅ Nonce replay protection
- ✅ Kafka message publishing with correct schema
- ✅ End-to-end latency and throughput

**Next Steps**:
- Load testing (thousands of messages/second)
- Multi-tenant device separation
- Error scenario handling (malformed JSON, wrong tenant_id, etc.)
- Horizontal scaling (multiple ingestion service instances with Kafka consumer groups)

**⚠️ Production Security Note:**
Before deploying to production, ensure you:
1. Set `InsecureSkipVerify: false` in `services/MQTT.service.go` to enable proper TLS certificate validation
2. Generate proper TLS certificates with SANs covering your broker's hostname(s)
3. Configure all MQTT clients to verify the broker's certificate against a trusted CA

The current configuration uses `InsecureSkipVerify: true` which is acceptable for local development/testing but **insecure for production**.
