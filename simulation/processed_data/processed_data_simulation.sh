#!/usr/bin/env bash
# Processed Data -> Kafka -> ML Service Pipeline
# Usage: ./processed_data_simulation.sh [--loop] [--delay SECONDS] [--kafka-brokers BROKERS]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Parse arguments
LOOP_FLAG=""
DELAY="1.0"
KAFKA_BROKERS="localhost:9092"
KAFKA_TOPIC="processed-data"
ML_SERVICE_URL="http://localhost:8004/processed-features"

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
        --kafka-brokers)
            KAFKA_BROKERS="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--loop] [--delay SECONDS] [--kafka-brokers BROKERS]"
            exit 1
            ;;
    esac
done

CSV_PATH="$PROJECT_ROOT/data/processed/bearing_features_sample.csv"
LOG_PATH="$PROJECT_ROOT/logs/simulation_predictions.jsonl"
BRIDGE_PID=""

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    if [ -n "$BRIDGE_PID" ] && kill -0 "$BRIDGE_PID" 2>/dev/null; then
        echo "Stopping bridge consumer (PID: $BRIDGE_PID)..."
        kill "$BRIDGE_PID" 2>/dev/null || true
        wait "$BRIDGE_PID" 2>/dev/null || true
    fi
    echo "Done."
}
trap cleanup EXIT

echo "=========================================="
echo "Processed Data -> Kafka -> ML Service Pipeline"
echo "=========================================="
echo "Kafka Brokers:  $KAFKA_BROKERS"
echo "Kafka Topic:    $KAFKA_TOPIC"
echo "ML Service:     $ML_SERVICE_URL"
echo "CSV Path:       $CSV_PATH"
echo "Delay:          ${DELAY}s between messages"
echo "Loop Mode:      $([ -n "$LOOP_FLAG" ] && echo 'enabled' || echo 'disabled')"
echo "Log File:       $LOG_PATH"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

# Check Kafka connectivity
echo "Checking Kafka..."
if ! docker compose -f "$PROJECT_ROOT/docker-compose.yml" ps kafka 2>/dev/null | grep -q "Up"; then
    echo "✗ Kafka is not running. Start it with: docker compose up -d kafka"
    exit 1
fi
echo "✓ Kafka is running"

# Check ML service
echo "Checking ML Service..."
if ! curl -s "http://localhost:8004/health" >/dev/null 2>&1; then
    echo "✗ ML Service is not reachable at http://localhost:8004"
    echo "  Start it with: docker compose up -d ml-service"
    exit 1
fi
echo "✓ ML Service is healthy"
echo ""

# Start the bridge consumer in background
echo "Starting Kafka -> ML Service bridge..."
cd "$PROJECT_ROOT"
KAFKA_BROKERS="$KAFKA_BROKERS" \
KAFKA_TOPIC="$KAFKA_TOPIC" \
ML_SERVICE_URL="$ML_SERVICE_URL" \
    python3 simulation/processed_data/processed_data_bridge.py &
BRIDGE_PID=$!
echo "Bridge started (PID: $BRIDGE_PID)"
sleep 3
echo ""

# Run the simulator to send data to Kafka
echo "Starting processed data simulation..."
echo "=========================================="
echo ""

python3 simulation/processed_data/replay_processed_data.py \
    --csv "$CSV_PATH" \
    --kafka \
    --kafka-brokers "$KAFKA_BROKERS" \
    --kafka-topic "$KAFKA_TOPIC" \
    --delay "$DELAY" \
    --log-file "$LOG_PATH" \
    $LOOP_FLAG

echo ""
echo "=========================================="
echo "Simulation complete!"
echo "Predictions logged to: $LOG_PATH"
echo ""
echo "Check ML service database:"
echo "  docker compose exec ml-service python -c \\"
echo "    'from db_models import get_db_engine; from sqlalchemy import text; \\"
echo "     engine = get_db_engine(\"postgresql://postgres:postgres@postgres:5432/predictions\"); \\"
echo "     print(list(engine.connect().execute(text(\"SELECT device_id FROM processed_features LIMIT 5\"))))'"
