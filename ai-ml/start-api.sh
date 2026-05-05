#!/usr/bin/env bash
# Start the Bearing Anomaly Prediction API with automatic DB setup
# Usage: ./start-api.sh [--no-db-setup]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKIP_DB_SETUP=${1:-""}

# Configuration
POSTGRES_CONTAINER_FILTER="ancestor=postgres:18"
POSTGRES_USER="postgres"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
PREDICTIONS_USER="predictions_user"
PREDICTIONS_PASSWORD="${PREDICTIONS_PASSWORD:-predictions_password}"
PREDICTIONS_DB="predictions"
POSTGRES_PORT="5433"
API_HOST="0.0.0.0"
API_PORT="${API_PORT:-8000}"

echo "=========================================="
echo "Bearing Anomaly Prediction API - Setup"
echo "=========================================="

# Step 1: Ensure Postgres is running
echo ""
echo "[1/4] Checking if Postgres is running..."
CONTAINER=$(docker ps --filter "$POSTGRES_CONTAINER_FILTER" --format "{{.Names}}" | head -n1)

if [ -z "$CONTAINER" ]; then
    echo "  → Postgres container not running. Starting..."
    cd "$PROJECT_ROOT"
    docker compose up -d postgres
    
    # Wait for Postgres to be healthy
    echo "  → Waiting for Postgres to be healthy (max 30s)..."
    for i in {1..30}; do
        if docker exec "$CONTAINER" pg_isready -U "$POSTGRES_USER" >/dev/null 2>&1; then
            echo "  ✓ Postgres is healthy"
            break
        fi
        if [ $i -eq 30 ]; then
            echo "  ✗ Postgres failed to start"
            exit 1
        fi
        sleep 1
    done
    
    # Re-fetch container name after docker compose up
    CONTAINER=$(docker ps --filter "$POSTGRES_CONTAINER_FILTER" --format "{{.Names}}" | head -n1)
else
    echo "  ✓ Postgres is running (container: $CONTAINER)"
fi

# Step 2: Create user and database (unless --no-db-setup)
if [ "$SKIP_DB_SETUP" != "--no-db-setup" ]; then
    echo ""
    echo "[2/4] Setting up database user and database..."
    
    # Create role
    docker exec -i "$CONTAINER" psql -U "$POSTGRES_USER" -d postgres -c \
        "DO \$$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '$PREDICTIONS_USER') THEN CREATE ROLE $PREDICTIONS_USER LOGIN PASSWORD '$PREDICTIONS_PASSWORD'; END IF; END \$$;" \
        >/dev/null 2>&1
    echo "  ✓ Role '$PREDICTIONS_USER' ensured"
    
    # Create database
    docker exec -i "$CONTAINER" createdb -U "$POSTGRES_USER" -O "$PREDICTIONS_USER" "$PREDICTIONS_DB" 2>/dev/null || true
    echo "  ✓ Database '$PREDICTIONS_DB' ensured"
else
    echo ""
    echo "[2/4] Skipping database setup (--no-db-setup)"
fi

# Step 3: Activate venv and check dependencies
echo ""
echo "[3/4] Setting up Python environment..."
cd "$SCRIPT_DIR"

if [ ! -d ".venv" ]; then
    echo "  → Creating virtual environment..."
    python3 -m venv .venv
fi

source .venv/bin/activate
echo "  ✓ Virtual environment activated"

# Check required packages
if ! python3 -c "import fastapi" 2>/dev/null; then
    echo "  → Installing dependencies from requirements.txt..."
    pip install -q -r requirements.txt
fi
echo "  ✓ Dependencies are installed"

# Step 4: Start API
echo ""
echo "[4/4] Starting Prediction API..."
echo "  Host: $API_HOST"
echo "  Port: $API_PORT"
echo "  Database: postgresql://$PREDICTIONS_USER:***@localhost:$POSTGRES_PORT/$PREDICTIONS_DB"
echo ""
echo "=========================================="
echo "API Documentation:"
echo "  • Swagger UI: http://localhost:$API_PORT/docs"
echo "  • ReDoc:      http://localhost:$API_PORT/redoc"
echo "  • Health:     http://localhost:$API_PORT/health"
echo "=========================================="
echo ""

cd "$SCRIPT_DIR/src"
export DATABASE_URL="postgresql://$PREDICTIONS_USER:$PREDICTIONS_PASSWORD@localhost:$POSTGRES_PORT/$PREDICTIONS_DB"
export API_HOST=$API_HOST
export API_PORT=$API_PORT

python3 prediction_api.py
