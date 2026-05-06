# Running the ML Prediction Pipeline

This guide explains how to run the Bearing Anomaly Prediction API and the workflow integration test.

## Prerequisites

- Docker and Docker Compose installed
- Python 3.10+ with virtual environment already set up (`.venv/`)
- PostgreSQL running in Docker (via docker-compose)
- Port 8000 available for the API server
- Port 5433 available for PostgreSQL

## Quick Start

### Simulation format toggle

For raw MQTT simulation with switchable payload schemas, use `simulation/raw_telemetry_engine.py` with:

- `--format new` for `device_name`, scalar vibration/temp fields
- `--format old` for legacy fields (`device_id`, `asset_id`, vibration arrays, temperature_bearing/atmospheric)

See `simulation/README.md` for commands and Kafka forwarding compatibility notes.

### 1. Start PostgreSQL

From the project root directory:

```bash
docker compose up -d postgres
```

This starts PostgreSQL on `localhost:5433` with the default user `postgres:postgres`.

**Verify it's running:**
```bash
docker ps | grep postgres
```

### 2. Activate Python Virtual Environment

```bash
cd ai-ml
source .venv/bin/activate
```

### 3. Create the Predictions Database

The API expects a `predictions` database with a `predictions_user` role. Create it:

```bash
docker exec -i anomaly_detection_project-postgres-1 psql -U postgres -d postgres \
  -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'predictions_user') THEN CREATE ROLE predictions_user LOGIN PASSWORD 'predictions_password'; END IF; END \$\$;"

docker exec -i anomaly_detection_project-postgres-1 createdb -U postgres -O predictions_user predictions 2>/dev/null || true
```

### 4. Start the Prediction API

From the `ai-ml/src` directory:

```bash
cd src
DATABASE_URL="postgresql://predictions_user:predictions_password@localhost:5433/predictions" \
python prediction_api.py
```

You should see output like:
```
INFO:     Uvicorn running on http://0.0.0.0:8000
INFO:     Started reloader process [PID] using StatReload
```

**Keep this terminal open** — the API runs in the foreground.

### 5. Run the Integration Test

In a **new terminal**, from the `ai-ml/tests` directory:

```bash
cd ai-ml/tests
source ../.venv/bin/activate
python workflow_integration_test.py
```

The test will:
- Check API health
- Save processed features
- Run ML predictions
- Review predictions
- Check retraining eligibility
- Get model versions
- Output results to console

**Expected output** (final lines):
```
================================================================================
WORKFLOW COMPLETE
================================================================================
```

## File Structure

```
ai-ml/
├── src/
│   ├── prediction_api.py      # FastAPI application
│   ├── prediction_module.py    # ML predictor class
│   ├── db_models.py            # SQLAlchemy models
│   ├── prediction_service.py   # Prediction business logic
│   ├── review_service.py       # Review/feedback logic
│   ├── retraining_service.py   # Retraining eligibility
│   └── ...
├── tests/
│   ├── workflow_integration_test.py  # Full end-to-end test
│   ├── test_prediction.py            # Smoke test for model
│   └── test_ml_services.py           # Unit tests
├── artifacts/
│   └── models/
│       ├── vibration_isolation_forest.pkl
│       ├── feature_columns.json
│       └── anomaly_thresholds.json
├── data/
│   └── processed/
│       └── bearing_features_labeled.csv
├── .env.example        # Environment variable template
└── requirements.txt    # Python dependencies
```

## Environment Configuration

The API loads configuration from environment variables. Create a `.env` file in `ai-ml/`:

```bash
# From the .env.example template
DATABASE_URL=postgresql://predictions_user:predictions_password@localhost:5433/predictions
API_HOST=0.0.0.0
API_PORT=8000
API_DEBUG=True
LOG_LEVEL=INFO
```

Or simply export them before running `prediction_api.py` (as shown in step 4 above).

## API Endpoints

Once running, you can test endpoints manually:

**Health Check:**
```bash
curl http://localhost:8000/health
```

**API Documentation (Swagger UI):**
```
http://localhost:8000/docs
```

**API Documentation (ReDoc):**
```
http://localhost:8000/redoc
```

## Troubleshooting

### PostgreSQL Connection Refused
```
psycopg2.OperationalError: connection to server at "localhost" (127.0.0.1), port 5433 failed
```

**Fix:** Ensure PostgreSQL is running:
```bash
docker compose up -d postgres
# Wait 10 seconds for startup
sleep 10
```

### Database Role Authentication Failed
```
psycopg2.OperationalError: password authentication failed for user "user"
```

**Fix:** Re-create the `predictions_user` and `predictions` database (see step 3).

### Port 8000 Already in Use
```
Address already in use
```

**Fix:** Kill the existing process or use a different port:
```bash
lsof -i :8000
kill -9 <PID>

# Or change API_PORT in .env and adjust the test API_BASE
```

### Model Artifacts Not Found
```
FileNotFoundError: Could not find sample CSV...
```

**Fix:** Ensure model artifacts are at:
```
ai-ml/artifacts/models/vibration_isolation_forest.pkl
ai-ml/artifacts/models/feature_columns.json
ai-ml/artifacts/models/anomaly_thresholds.json
ai-ml/data/processed/bearing_features_labeled.csv
```

### scikit-learn Version Warning
```
InconsistentVersionWarning: Trying to unpickle estimator ... from version 1.6.1 when using version 1.7.2
```

**Info:** This is a non-fatal warning. The model was trained with sklearn 1.6.1 but the venv has 1.7.2. It works fine but you can suppress it by pinning sklearn in `requirements.txt`:

```bash
pip install 'scikit-learn==1.6.1'
pip freeze > requirements.txt
```

## Stopping the Services

### Stop API
Press `Ctrl+C` in the terminal running `prediction_api.py`.

### Stop PostgreSQL
```bash
docker compose down postgres
```

### Stop Everything (including volumes)
```bash
# Preserves data
docker compose down

# Removes volumes (deletes data)
docker compose down -v
```

## Running Tests Only (Without API)

Unit tests don't require the API:

```bash
cd ai-ml/tests
source ../.venv/bin/activate
python test_prediction.py   # Smoke test for model loading
python test_ml_services.py  # Unit tests for services
```

## Development Notes

- The API uses SQLAlchemy for ORM and GORM-style AutoMigrate for schema management
- Multi-tenancy is enforced at the query layer (tenant_id flows from Kafka/API into all DB queries)
- The predictor uses Isolation Forest for anomaly detection with 3 severity levels (normal, warning, critical)
- Retraining eligibility is based on minimum reviewed records (default: 100)
