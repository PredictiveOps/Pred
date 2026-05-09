#!/bin/bash
# Run MQTT simulation through the full pipeline: simulation -> MQTT -> ingestion -> Kafka
# Usage: ./run_simulation.sh [device_id] [format] [count] [rate] [--loop] [--delay SECONDS] [verbose]
# Examples:
#   ./run_simulation.sh 1 new 100 10                    # Single batch of 100 messages
#   ./run_simulation.sh --loop --delay 30 1 new 50 5   # Continuous mode, 50 messages every 30s at 5 msg/s

set -e

# Parse arguments
LOOP_FLAG=""
DELAY="60"  # Default delay between batches in seconds
DEVICE_ID=""
FORMAT=""
COUNT=""
RATE=""
VERBOSE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --loop)
            LOOP_FLAG="--loop"
            shift
            ;;
        --delay)
            DELAY="$2"
            shift 2
            ;;
        *)
            # Positional arguments - assign in order
            if [[ -z "$DEVICE_ID" ]]; then
                DEVICE_ID="$1"
            elif [[ -z "$FORMAT" ]]; then
                FORMAT="$1"
            elif [[ -z "$COUNT" ]]; then
                COUNT="$1"
            elif [[ -z "$RATE" ]]; then
                RATE="$1"
            else
                VERBOSE="$1"
            fi
            shift
            ;;
    esac
done

# Set defaults for empty positional arguments
DEVICE_ID="${DEVICE_ID:-1}"
FORMAT="${FORMAT:-new}"
COUNT="${COUNT:-100}"
RATE="${RATE:-10}"

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SIM_DIR="$PROJECT_ROOT/simulation"
PRIVATE_KEY="$PROJECT_ROOT/simulation/device-private.pem"
PUBLIC_KEY="$PROJECT_ROOT/simulation/device-public.pem"

# Cleanup function
cleanup() {
    echo ""
    echo "Shutting down simulation..."
    # Kill any background processes if needed
    exit 0
}
trap cleanup EXIT INT TERM

echo "=== Pred Simulation Runner ==="
echo "Device ID: $DEVICE_ID"
echo "Format: $FORMAT"
echo "Count: $COUNT messages"
echo "Rate: $RATE msg/s"
echo "Loop Mode: $([ -n "$LOOP_FLAG" ] && echo 'enabled' || echo 'disabled')"
echo "Delay: ${DELAY}s between batches"
echo "Verbose: ${VERBOSE:-no}"
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
        --cafile ../mosquitto/certs/ca.crt \
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

if [ -n "$VERBOSE" ]; then
    PROGRESS_INTERVAL=0
    echo "Verbose mode enabled - showing message payloads"
else
    PROGRESS_INTERVAL=10
fi

# Function to run a single batch
run_simulation_batch() {
    echo ""
    echo "Starting batch of $COUNT messages..."
    cd "$SIM_DIR/raw_data"
    
    python3 raw_telemetry_engine.py \
        --device "$DEVICE_ID" \
        --format "$FORMAT" \
        --signed \
        --private-key "$PRIVATE_KEY" \
        --broker localhost \
        --port 8883 \
        --ca-cert "$PROJECT_ROOT/mosquitto/certs/ca.crt" \
        --rate "$RATE" \
        --count "$COUNT" \
        --username pred-device \
        --password dev-device-password \
        --progress-interval "$PROGRESS_INTERVAL" \
        ${VERBOSE:+--verbose}
    
    echo "Batch completed. Messages sent: $COUNT"
}

# Main execution loop
if [ -n "$LOOP_FLAG" ]; then
    echo "Continuous mode enabled. Running batches with ${DELAY}s delay."
    echo "Press Ctrl+C to stop."
    
    while true; do
        run_simulation_batch
        echo "Waiting ${DELAY}s before next batch..."
        sleep "$DELAY"
    done
else
    # Single batch execution
    run_simulation_batch
fi

echo ""
echo "=== Simulation Complete ==="
echo ""
echo "To verify data in Kafka, run:"
echo "  docker compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic sensor_data --from-beginning --max-messages 5"
