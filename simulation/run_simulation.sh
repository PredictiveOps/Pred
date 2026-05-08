#!/bin/bash
# Run MQTT simulation through the full pipeline: simulation -> MQTT -> ingestion -> Kafka
# Usage: ./run_simulation.sh [device_id] [format] [count]

set -e

DEVICE_ID="${1:-1}"
FORMAT="${2:-new}"
COUNT="${3:-100}"
RATE="${4:-10}"

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SIM_DIR="$PROJECT_ROOT/simulation"
PRIVATE_KEY="$SIM_DIR/device-private.pem"
PUBLIC_KEY="$SIM_DIR/device-public.pem"

echo "=== Pred Simulation Runner ==="
echo "Device ID: $DEVICE_ID"
echo "Format: $FORMAT"
echo "Count: $COUNT messages"
echo "Rate: $RATE msg/s"
echo ""

# 1. Generate keys if they don't exist
if [ ! -f "$PRIVATE_KEY" ]; then
    echo "[1/4] Generating device keypair..."
    openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$PRIVATE_KEY" 2>/dev/null
    openssl pkey -in "$PRIVATE_KEY" -pubout -out "$PUBLIC_KEY" 2>/dev/null
    echo "      Keys created: device-private.pem, device-public.pem"
else
    echo "[1/4] Using existing keypair"
fi

# 2. Register device via HTTP API
echo "[2/4] Registering device $DEVICE_ID..."
curl -s -X POST http://localhost:2500/devices/register \
    -H 'Content-Type: application/json' \
    -d "{\"device_id\": $DEVICE_ID, \"tenant_id\": 1}" || {
    echo "      Warning: Device registration may already exist or ingestion service not running"
}

# 3. Register public key via MQTT
echo "[3/4] Registering public key via MQTT..."
# Format public key as single-line JSON string
PUBKEY=$(awk 'NF {sub(/\r/, ""); printf "%s\\n", $0}' "$PUBLIC_KEY")

# Check if mosquitto is accessible
if docker compose ps mosquitto &>/dev/null; then
    docker compose exec -T mosquitto mosquitto_pub \
        -h localhost -p 8883 \
        --cafile /mosquitto/config/certs/ca.crt \
        -u pred-device -P dev-device-password \
        -i "$DEVICE_ID" \
        -t "devices/$DEVICE_ID/registration" \
        -m "{\"public_key\": \"$PUBKEY\"}" || {
        echo "      Warning: MQTT registration may have failed (device might already be registered)"
    }
else
    echo "      Warning: Mosquitto container not running. Ensure docker compose is up."
fi

# 4. Run simulation
echo "[4/4] Starting simulation..."
echo ""
cd "$SIM_DIR"

python3 raw_telemetry_engine.py \
    --device "$DEVICE_ID" \
    --format "$FORMAT" \
    --signed \
    --private-key "$PRIVATE_KEY" \
    --broker localhost \
    --port 8883 \
    --rate "$RATE" \
    --count "$COUNT" \
    --username pred-device \
    --password dev-device-password \
    --progress-interval 10

echo ""
echo "=== Simulation Complete ==="
echo ""
echo "To verify data in Kafka, run:"
echo "  docker compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic sensor_data --from-beginning --max-messages 5"
