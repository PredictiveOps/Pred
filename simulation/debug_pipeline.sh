#!/bin/bash
# Debug script to diagnose why data isn't reaching Kafka

echo "=== Pipeline Debug Diagnostic ==="
echo ""

# 1. Check containers are running
echo "[1] Checking container status..."
docker compose ps | grep -E "(mosquitto|ingestion|kafka)" || true
echo ""

# 2. Check ingestion service logs for errors
echo "[2] Recent ingestion service logs (last 50 lines)..."
docker compose logs --tail=50 ingestion-service 2>&1 | tail -50
echo ""

# 3. Check if device is registered in database
echo "[3] Checking device registration in database..."
docker compose exec postgres psql -U postgres -d ingestion -c "SELECT device_id, tenant_id, is_active, public_key IS NOT NULL as has_key FROM devices;" 2>/dev/null || echo "      Could not query database"
echo ""

# 4. Test MQTT connection manually
echo "[4] Testing MQTT with a simple message..."
docker compose exec mosquitto mosquitto_pub \
    -h localhost -p 8883 \
    --cafile /mosquitto/config/certs/ca.crt \
    -u pred-device -P dev-device-password \
    -i test-client \
    -t 'devices/1/data' \
    -m '{"timestamp":'$(date +%s)',"nonce":"test-1","data":{"device_name":"test","timestamp":"2024-01-01T00:00:00Z","vibration_x":0.5,"vibration_y":0.4,"temp_motor":70,"temp_atmospheric":20},"signature":"invalid"}' \
    2>&1 || echo "      MQTT publish failed (this is expected if using invalid signature)"
echo ""

# 5. Check if kafka topic exists
echo "[5] Checking Kafka topics..."
docker compose exec kafka kafka-topics --bootstrap-server localhost:9092 --list 2>/dev/null || echo "      Could not list topics"
echo ""

# 6. Check ingestion service environment
echo "[6] Checking ingestion service environment variables..."
docker compose exec ingestion-service env | grep -E "(MQTT|KAFKA)" 2>/dev/null || echo "      Could not get environment"
echo ""

# 7. Test HTTP API
echo "[7] Testing ingestion HTTP API..."
curl -s http://localhost:2500/health 2>&1 || echo "      HTTP API not responding"
echo ""

# 8. Check redis for device state
echo "[8] Checking Redis for device state..."
docker compose exec redis redis-cli keys "*" 2>/dev/null | head -10 || echo "      Could not query Redis"
echo ""

echo "=== Diagnostic Complete ==="
echo ""
echo "Common issues:"
echo "  - 'signature verification failed' = Device not registered or wrong key"
echo "  - 'failed to parse sensor data' = MQTT_PAYLOAD_FORMAT mismatch"
echo "  - 'device is inactive' = Device exists but is_active=false"
echo "  - 'device has no public key' = Public key not registered via MQTT"
echo "  - No logs at all = Ingestion not subscribed to MQTT topic"
