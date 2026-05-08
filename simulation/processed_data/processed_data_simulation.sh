#!/usr/bin/env bash
# =============================================================================
# Full Prediction Pipeline Simulation
# =============================================================================
# Pipeline Flow:
#   Simulation (CSV) -> Kafka (processed-data) -> ML Service (/predict/vibration)
#       -> Kafka (predictions) -> prediction-persister -> Database (ALL predictions)
#                            -> alert-notifier -> Kafka (alerts) [warning/critical only]
# =============================================================================
# Usage: ./processed_data_simulation.sh [--loop] [--delay SECONDS] [--kafka-brokers BROKERS]
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Parse arguments
LOOP_FLAG=""
DELAY="1.0"
KAFKA_BROKERS="localhost:9092"
KAFKA_TOPIC="processed-data"
ML_SERVICE_URL="http://localhost:8004/predict/vibration"

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
echo "Full Prediction Pipeline Simulation"
echo "=========================================="
echo ""
echo "Pipeline Architecture:"
echo "  [Simulation] --(CSV)--> [Kafka: processed-data]"
echo "       |"
echo "       v"
echo "  [Bridge] --(HTTP)--> [ML Service: /predict/vibration]"
echo "       |"
echo "       v"
echo "  [Kafka: predictions]"
echo "       |"
echo "       +---> [prediction-persister] --> [Database: ALL predictions]"
echo "       |"
echo "       +---> [alert-notifier] --> [Kafka: alerts] (warning/critical only)"
echo ""
echo "=========================================="
echo "Configuration:"
echo "  Kafka Brokers:  $KAFKA_BROKERS"
echo "  Input Topic:    $KAFKA_TOPIC"
echo "  ML Service:     $ML_SERVICE_URL"
echo "  CSV Path:       $CSV_PATH"
echo "  Delay:          ${DELAY}s between messages"
echo "  Loop Mode:      $([ -n "$LOOP_FLAG" ] && echo 'enabled' || echo 'disabled')"
echo "  Log File:       $LOG_PATH"
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

# Check prediction-persister
echo "Checking prediction-persister..."
if docker compose -f "$PROJECT_ROOT/docker-compose.yml" ps prediction-persister 2>/dev/null | grep -q "Up"; then
    echo "✓ prediction-persister is running (will store ALL predictions to database)"
else
    echo "⚠ prediction-persister is not running (predictions won't be stored to DB)"
    echo "  Start with: docker compose up -d prediction-persister"
fi

# Check alert-notifier
echo "Checking alert-notifier..."
if docker compose -f "$PROJECT_ROOT/docker-compose.yml" ps alert-notifier 2>/dev/null | grep -q "Up"; then
    echo "✓ alert-notifier is running (will filter warning/critical to alerts topic)"
else
    echo "⚠ alert-notifier is not running (alerts won't be generated)"
    echo "  Start with: docker compose up -d alert-notifier"
fi
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
echo "=========================================="
echo "Verify Pipeline:"
echo "=========================================="
echo ""
echo "1. Check predictions Kafka topic:"
echo "   docker exec -it \$(docker ps -qf 'ancestor=apache/kafka') \\"
echo "     /opt/kafka/bin/kafka-console-consumer.sh \\"
echo "     --bootstrap-server localhost:9092 --topic predictions --from-beginning --max-messages 5"
echo ""
echo "2. Check alerts Kafka topic (warning/critical only):"
echo "   docker exec -it \$(docker ps -qf 'ancestor=apache/kafka') \\"
echo "     /opt/kafka/bin/kafka-console-consumer.sh \\"
echo "     --bootstrap-server localhost:9092 --topic alerts --from-beginning --max-messages 5"
echo ""
echo "3. Check predictions in database:"
echo "   docker exec anomaly_detection_project-postgres-1 \\"
echo "     psql -U postgres -d predictions \\"
echo "     -c 'SELECT prediction_id, predicted_status, created_at FROM predictions ORDER BY created_at DESC LIMIT 10;'"
echo ""
echo "4. Check prediction-persister logs:"
echo "   docker compose logs prediction-persister --tail 20"
echo ""
echo "5. Check alert-notifier logs:"
echo "   docker compose logs alert-notifier --tail 20"
echo ""
echo "=========================================="
