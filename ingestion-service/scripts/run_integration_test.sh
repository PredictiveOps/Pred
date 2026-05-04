#!/bin/bash
# set -e  # Temporarily disabled to see all test results

echo "=========================================="
echo "Ingestion Service Integration Test"
echo "=========================================="
echo ""

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test state
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function to report results
test_result() {
    local test_name=$1
    local exit_code=$2
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $test_name"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗${NC} $test_name"
        ((TESTS_FAILED++))
    fi
}

echo "Step 1: Verify infrastructure..."
docker compose ps > /dev/null 2>&1
test_result "Docker Compose services running" $?

echo ""
echo "Step 2: Verify HTTP API health..."
curl -s http://localhost:2500/health | jq . > /dev/null 2>&1
test_result "Ingestion service health check" $?

echo ""
echo "Step 3: Generate test keypair..."
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 \
  -out /tmp/test-device-private.pem > /dev/null 2>&1
test_result "Generate ECDSA keypair" $?

openssl pkey -in /tmp/test-device-private.pem -pubout \
  -out /tmp/test-device-public.pem > /dev/null 2>&1
test_result "Extract public key" $?

echo ""
echo "Step 4: Create device via HTTP..."
DEVICE_ID=$(($(date +%s) + RANDOM))
TENANT_ID=1
DEVICE_RESPONSE=$(curl -s -X POST http://localhost:2500/devices/register \
  -H 'Content-Type: application/json' \
  -d "{\"device_id\": $DEVICE_ID, \"tenant_id\": $TENANT_ID}")

echo "$DEVICE_RESPONSE" | grep -q "registration_status" 
test_result "Create device (HTTP POST)" $?

echo ""
echo "Step 5: Register device public key via MQTT..."
REGISTRATION_JSON=$(jq -Rs '{public_key: .}' /tmp/test-device-public.pem)

docker run --rm --network host -v /tmp:/tmp eclipse-mosquitto:2 mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /tmp/ca.crt \
  --insecure \
  -u pred-device \
  -P dev-device-password \
  -i $DEVICE_ID \
  -t "devices/$DEVICE_ID/registration" \
  -m "$REGISTRATION_JSON" > /dev/null 2>&1
test_result "Publish registration message (MQTT)" $?

sleep 2

# Verify public key was stored
docker compose exec postgres psql \
  -U postgres \
  -d ingestion \
  -c "SELECT public_key FROM devices WHERE device_id = $DEVICE_ID" 2>/dev/null | grep -q "BEGIN PUBLIC KEY"
test_result "Device public key stored in database" $?

echo ""
echo "Step 6: Generate and sign telemetry payload..."
python3 ingestion-service/scripts/sign_mqtt_payload.py /tmp/test-device-private.pem > /tmp/test-payload.json 2>/dev/null
test_result "Generate signed payload" $?

SIGNED_PAYLOAD=$(cat /tmp/test-payload.json)
echo "$SIGNED_PAYLOAD" | jq . > /dev/null 2>&1
test_result "Validate JSON signature format" $?

echo ""
echo "Step 7: Publish telemetry via MQTT..."
docker run --rm --network host -v /tmp:/tmp eclipse-mosquitto:2 mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /tmp/ca.crt \
  --insecure \
  -u pred-device \
  -P dev-device-password \
  -i $DEVICE_ID \
  -t "devices/$DEVICE_ID/data" \
  -m "$SIGNED_PAYLOAD" > /dev/null 2>&1
test_result "Publish signed telemetry (MQTT)" $?

sleep 2

echo ""
echo "Step 8: Verify message in Kafka..."
KAFKA_MESSAGE=$(docker compose exec kafka bash -c "/opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic sensor_data --from-beginning --max-messages 50 --timeout-ms 8000" 2>/dev/null | grep -E '\"device_id\":\s*'$DEVICE_ID | head -1 || echo "")

echo "$KAFKA_MESSAGE" | jq . > /dev/null 2>&1
test_result "Consume message from Kafka" $?

echo "$KAFKA_MESSAGE" | grep -q "device_id" 2>/dev/null
test_result "Kafka message contains device_id" $?

echo ""
echo "Step 9: Verify Kafka payload structure..."
echo "$KAFKA_MESSAGE" | jq -e '.device_id == '$DEVICE_ID' and .mode == "normal" and (.v_rms - 1.23 | abs) < 0.001' > /dev/null 2>&1
test_result "Kafka payload has correct sensor data" $?

echo ""
echo "Step 10: Test replay protection..."
docker run --rm --network host -v /tmp:/tmp eclipse-mosquitto:2 mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /tmp/ca.crt \
  --insecure \
  -u pred-device \
  -P dev-device-password \
  -i $DEVICE_ID \
  -t "devices/$DEVICE_ID/data" \
  -m "$SIGNED_PAYLOAD" > /dev/null 2>&1

sleep 1

# Check if replay protection worked by testing if a second message appears in Kafka
# (If replay protection works, there should be only one message)
sleep 1
KAFKA_MESSAGES=$(docker compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic sensor_data \
  --from-beginning \
  --timeout-ms 1000 2>/dev/null | wc -l)
if [ "$KAFKA_MESSAGES" -le 1 ]; then
  test_result "Replay protection (nonce rejection)" 0
else
  test_result "Replay protection (nonce rejection)" 1
fi

echo ""
echo "=========================================="
echo "Test Results:"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo "=========================================="

if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
fi
exit 0
