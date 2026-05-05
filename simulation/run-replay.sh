#!/usr/bin/env bash
# Replay processed bearing features and get predictions from the API
# Usage: ./run-replay.sh [--loop] [--delay SECONDS] [--tenant TENANT_ID]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse arguments
LOOP_FLAG=""
DELAY="1.0"
TENANT_ID="demo_tenant"

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
        --tenant)
            TENANT_ID="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--loop] [--delay SECONDS] [--tenant TENANT_ID]"
            exit 1
            ;;
    esac
done

API_BASE_URL="http://localhost:8000"
CSV_PATH="data/processed/bearing_features_sample.csv"
LOG_PATH="logs/simulation_predictions.jsonl"

echo "=========================================="
echo "Bearing Anomaly Replay Simulation"
echo "=========================================="
echo "API Base URL: $API_BASE_URL"
echo "CSV Path:    $CSV_PATH"
echo "Delay:       ${DELAY}s between predictions"
echo "Tenant ID:   $TENANT_ID"
echo "Loop Mode:   $([ -n "$LOOP_FLAG" ] && echo 'enabled' || echo 'disabled')"
echo "Log File:    $LOG_PATH"
echo ""

# Check if API is running
echo "Checking if API is reachable..."
if ! curl -s "$API_BASE_URL/health" >/dev/null 2>&1; then
    echo "✗ API is not reachable at $API_BASE_URL"
    echo "  Start the API first: ./start-api.sh"
    exit 1
fi
echo "✓ API is healthy"
echo ""

# Activate venv (located in ai-ml/)
AI_ML_DIR="$PROJECT_ROOT/ai-ml"
if [ ! -d "$AI_ML_DIR/.venv" ]; then
    echo "✗ Virtual environment not found at $AI_ML_DIR/.venv. Run ./start-api.sh first."
    exit 1
fi
source "$AI_ML_DIR/.venv/bin/activate"

# Run replay with save-then-predict
echo "Starting replay (save features → predict)..."
echo "=========================================="
echo ""

cd "$PROJECT_ROOT"
python3 simulation/replay_processed_data.py \
    --csv "$CSV_PATH" \
    --endpoint "$API_BASE_URL/predict" \
    --delay "$DELAY" \
    --log-file "$LOG_PATH" \
    --save-then-predict \
    $LOOP_FLAG

echo ""
echo "=========================================="
echo "Replay complete!"
echo "Predictions logged to: $LOG_PATH"
echo ""
echo "To view results:"
echo "  tail -f $LOG_PATH | jq"
