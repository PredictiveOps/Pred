# Reproducible AI Service Setup - Complete ✓

## What Was Created

I've implemented a fully reproducible, one-command setup process for the Bearing Anomaly Prediction AI service and simulation. Here are the deliverables:

### 1. **Setup Scripts** (Automated)

#### `ai-ml/start-api.sh` ✓
A single script that handles complete API setup:
- ✓ Starts Postgres (if not running)
- ✓ Waits for Postgres to be healthy
- ✓ Creates `predictions_user` role
- ✓ Creates `predictions` database
- ✓ Activates Python virtual environment
- ✓ Installs dependencies (if missing)
- ✓ Starts FastAPI on port 8000

**Run it:**
```bash
./ai-ml/start-api.sh
```

**Output:**
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
  ✓ Uvicorn running on http://0.0.0.0:8000
```

#### `simulation/run-replay.sh` ✓
A script to run the bearing features simulation:
- ✓ Verifies API is running
- ✓ Loads CSV features
- ✓ POSTs features to `/processed-features` (saves to DB)
- ✓ Calls `/predict` to get model predictions
- ✓ Logs all results to `logs/simulation_predictions.jsonl`

**Run it** (in another terminal):
```bash
./simulation/run-replay.sh [--delay SECONDS] [--loop] [--tenant TENANT_ID]
```

**Example:** 
```bash
./simulation/run-replay.sh --delay 0.5      # Replay with 0.5s between predictions
./simulation/run-replay.sh --loop           # Replay indefinitely
./simulation/run-replay.sh --tenant acme    # Use custom tenant
```

### 2. **Enhanced Simulation Script**

#### `simulation/replay_processed_data.py` ✓
Updated with `--save-then-predict` flag:
- ✓ POSTs processed features to `/processed-features` endpoint
- ✓ Automatically calls `/predict` with latest saved features
- ✓ Handles multi-tenant predictions (configurable tenant_id)
- ✓ Logs responses in JSONL format

### 3. **Documentation**

#### `AI_SERVICE_SETUP.md` ✓
Comprehensive guide covering:
- Quick start (one command)
- Step-by-step manual setup
- All CLI options and examples
- Troubleshooting guide
- Environment variable reference
- Manual command alternatives
- Cleanup procedures

---

## Usage Summary

### **Terminal 1: Start the API**
```bash
cd /path/to/Anomaly_detection_project
./ai-ml/start-api.sh
```

**Keep running.** This starts the API on `http://localhost:8000`

### **Terminal 2: Run the Simulation**
```bash
cd /path/to/Anomaly_detection_project
./simulation/run-replay.sh --delay 0.5
```

This will:
1. ✓ Check API is healthy
2. ✓ Load 60 rows from the bearing features CSV
3. ✓ Save each feature row to the database
4. ✓ Get a prediction from the model
5. ✓ Log results to `logs/simulation_predictions.jsonl`

### **Terminal 3: View Results (Optional)**
```bash
# Watch predictions in real-time
tail -f logs/simulation_predictions.jsonl | jq '.response.response | {status: .predicted_status, anomaly_score, severity: .severity_level}'

# Count predictions by status
cat logs/simulation_predictions.jsonl | jq -r '.response.response.predicted_status' | sort | uniq -c
```

---

## What's Working

✓ **API Setup:** Postgres user/DB auto-creation  
✓ **Venv Management:** Auto-activation and dependency installation  
✓ **Predictions:** Model runs and returns valid results  
✓ **Logging:** JSONL format with full request/response  
✓ **Multi-tenant:** Works with configurable tenant IDs  
✓ **Simulation:** Loads CSV, saves features, gets predictions  

---

## Key Features

### Idempotent Setup
- Can run `./start-api.sh` multiple times safely
- Skips already-created database roles/databases
- Venv and dependencies install only if missing

### No Manual Database Commands
- No need to manually create roles or databases
- No need to export complex DATABASE_URL strings
- All handled by the scripts

### Flexible Simulation
- Configurable delays between predictions
- Optional loop mode for continuous testing
- Custom tenant IDs for multi-tenant testing
- Full JSONL logging with request/response

### Clean Shutdown
- Press `Ctrl+C` in either terminal to stop
- No orphaned processes or zombie containers
- Safe to restart immediately

---

## Sample Output

**API Response (from logs/simulation_predictions.jsonl):**
```json
{
  "sent_at": "2026-05-05T17:22:35.555966+00:00",
  "row_index": 0,
  "request": {
    "device_id": "demo_device_001",
    "asset_id": "bearing_motor_001",
    "features": { ... }
  },
  "response_status": 200,
  "response": {
    "prediction_id": "bcb1bca2-0fb7-4a7c-9a0f-1914c955e348",
    "tenant_id": "demo_tenant",
    "predicted_status": "normal",
    "anomaly_score": -0.0683,
    "severity_level": 1,
    "recommended_action": "Machine condition is normal. Continue regular monitoring."
  }
}
```

---

## Files Created/Modified

| File | Purpose |
|------|---------|
| `ai-ml/start-api.sh` | **NEW** — API startup automation |
| `simulation/run-replay.sh` | **NEW** — Simulation runner |
| `simulation/replay_processed_data.py` | **MODIFIED** — Added `--save-then-predict` support |
| `AI_SERVICE_SETUP.md` | **NEW** — Complete documentation |

---

## Next Steps

1. **Start the API:**
   ```bash
   ./ai-ml/start-api.sh
   ```

2. **In another terminal, run simulation:**
   ```bash
   ./simulation/run-replay.sh
   ```

3. **View results:**
   ```bash
   tail -f logs/simulation_predictions.jsonl | jq
   ```

All setup is now **fully automated and reproducible**. No manual database commands, no complex environment variables, no guessing. 🎯
