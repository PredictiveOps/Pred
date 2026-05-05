# Reproducible AI Service & Simulation Setup

This guide provides automated, repeatable steps to run the Bearing Anomaly Prediction API and simulation.

## Quick Start (One Command)

From the project root:

```bash
./ai-ml/start-api.sh
```

This will:
- Start Postgres (if not running)
- Wait for it to be healthy
- Create the `predictions_user` role and `predictions` database (if needed)
- Activate the Python venv
- Install dependencies (if missing)
- Start the FastAPI server on `http://localhost:8000`

Then in another terminal, run the simulation:

```bash
./simulation/run-replay.sh
```

This will:
- Check that the API is running
- Load the processed bearing features CSV
- POST each row to `/processed-features` (saves to DB)
- Call `/predict` to run the model
- Log results to `logs/simulation_predictions.jsonl`

## Detailed Setup

### Prerequisites

- Docker and Docker Compose installed
- Python 3.10+ available
- Port 5433 (Postgres), 8000 (API) available

### Step-by-Step

#### 1. Start the Prediction API

```bash
cd /path/to/Anomaly_detection_project
./ai-ml/start-api.sh
```

**What it does:**
- Ensures Postgres container is running
- Waits up to 30 seconds for Postgres to be healthy
- Creates `predictions_user` role with password `predictions_password`
- Creates `predictions` database owned by `predictions_user`
- Activates Python venv (creates if needed)
- Installs dependencies from `requirements.txt` (if needed)
- Starts FastAPI on `http://0.0.0.0:8000`

**Expected output:**
```
[1/4] Checking if Postgres is running...
  ✓ Postgres is running (container: ...)
[2/4] Setting up database user and database...
  ✓ Role 'predictions_user' ensured
  ✓ Database 'predictions' ensured
[3/4] Setting up Python environment...
  ✓ Virtual environment activated
  ✓ Dependencies are installed
[4/4] Starting Prediction API...
  ...
  Uvicorn running on http://0.0.0.0:8000
```

**Keep this terminal open.** The API runs in the foreground.

#### 2. Run the Simulation (in another terminal)

```bash
cd /path/to/Anomaly_detection_project
./simulation/run-replay.sh
```

**Options:**
- `--loop` — Replay indefinitely when CSV ends
- `--delay SECONDS` — Delay between predictions (default: 1.0)
- `--tenant TENANT_ID` — Use a different tenant ID (default: `demo_tenant`)

**Examples:**
```bash
# Replay once, 2-second delay between predictions
./simulation/run-replay.sh --delay 2.0

# Replay indefinitely, with 0.5-second delay
./simulation/run-replay.sh --delay 0.5 --loop

# Use a custom tenant
./simulation/run-replay.sh --tenant acme-corp
```

**Expected output:**
```
Checking if API is reachable...
✓ API is healthy

Starting replay (save features → predict)...
==========================================

--- Sent packet ---
{
  "device_id": "demo_device_001",
  "asset_id": "bearing_motor_001",
  ...
}
--- Prediction response from model ---
{
  "status_code": 200,
  "response": {
    "prediction_id": "...",
    "predicted_status": "critical",
    ...
  }
}
```

#### 3. View Results

Predictions are logged to `logs/simulation_predictions.jsonl` (one JSON per line):

```bash
# View last 10 predictions
tail -10 logs/simulation_predictions.jsonl | jq

# Stream live predictions (if replay is running with --loop)
tail -f logs/simulation_predictions.jsonl | jq '.response | {status_code, predicted_status}'

# Count critical predictions
cat logs/simulation_predictions.jsonl | jq -r '.response.response.predicted_status' | sort | uniq -c
```

## Troubleshooting

### API fails to start: "password authentication failed for user 'user'"

The API's default `DATABASE_URL` expects `predictions_user`. If the setup script didn't run:

```bash
# Re-run setup with fresh DB creation
./ai-ml/start-api.sh
```

Or manually set the correct URL:

```bash
cd ai-ml/src
DATABASE_URL="postgresql://predictions_user:predictions_password@localhost:5433/predictions" \
python prediction_api.py
```

### Postgres container won't start

```bash
# Check Docker status
docker ps | grep postgres

# View Postgres logs
docker logs <container_id>

# Force restart
docker compose down postgres
docker compose up -d postgres
```

### Replay says "API not reachable"

1. Check API is running: `curl http://localhost:8000/health`
2. Check for port conflict: `lsof -i :8000`
3. Verify network: `ping localhost:8000` or `telnet localhost 8000`

### No predictions in the log file

1. Check `logs/` directory exists: `mkdir -p logs`
2. Verify API responses are 200: tail the JSONL and check `status_code`
3. Check for DB connectivity errors in API terminal

## Environment Variables

Both scripts respect these env vars:

```bash
# Postgres credentials
POSTGRES_PASSWORD=my_password

# Predictions DB user
PREDICTIONS_PASSWORD=my_predictions_password

# API server
API_PORT=9000          # default: 8000
API_HOST=127.0.0.1     # default: 0.0.0.0

# Replay simulation
# (set inside run-replay.sh --tenant option or edit the script)
```

## Manual Commands (for reference)

If you prefer not to use the scripts:

```bash
# Start Postgres
docker compose up -d postgres

# Create predictions_user + database
CONTAINER=$(docker ps --filter "ancestor=postgres:18" --format "{{.Names}}" | head -n1)
docker exec -i "$CONTAINER" psql -U postgres -d postgres -c \
  "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'predictions_user') THEN CREATE ROLE predictions_user LOGIN PASSWORD 'predictions_password'; END IF; END \$$;"
docker exec -i "$CONTAINER" createdb -U postgres -O predictions_user predictions

# Start API
cd ai-ml
source .venv/bin/activate
cd src
DATABASE_URL="postgresql://predictions_user:predictions_password@localhost:5433/predictions" \
python prediction_api.py

# Run replay (in another terminal)
cd simulation
python3 replay_processed_data.py \
  --csv data/processed/bearing_features_sample.csv \
  --endpoint http://localhost:8000/predict \
  --delay 1.0 \
  --save-then-predict
```

## API Endpoints (for manual testing)

- **Health Check**: `curl http://localhost:8000/health`
- **Swagger Docs**: http://localhost:8000/docs
- **ReDoc**: http://localhost:8000/redoc
- **Save Features**: `POST /processed-features`
- **Make Prediction**: `POST /predict`
- **Get Pending Predictions**: `GET /predictions/pending?tenant_id=demo_tenant`

## Cleanup

To stop services:

```bash
# Stop API: Press Ctrl+C in the API terminal

# Stop Postgres (keeps data)
docker compose down postgres

# Stop everything (keeps data)
docker compose down

# Remove everything (delete data)
docker compose down -v
```
