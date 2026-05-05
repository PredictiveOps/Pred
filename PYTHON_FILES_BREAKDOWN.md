# AI-ML Python Files Breakdown & Cleanup Guide

## Overview

The `ai-ml/` directory contains **15 Python files** organized into three categories:
- **Core API & Services** (src/) — Production code
- **Data Pipeline** (root level) — Optional Kafka consumer
- **Tests** (tests/) — Testing and validation

---

## Core Production Files (`ai-ml/src/`)

### **1. prediction_api.py** ✅ REQUIRED
**Purpose:** FastAPI application server (the main entry point)

**Responsibilities:**
- HTTP endpoints for predictions, reviews, retraining
- Request validation with Pydantic models
- Database session management
- Service orchestration

**Used By:** Direct API calls, simulation, web frontend

**Can Remove?** **NO** — This is the main service you're running daily

---

### **2. prediction_module.py** ✅ REQUIRED
**Purpose:** ML model inference wrapper

**Responsibilities:**
- Loads the trained Isolation Forest model from disk
- Loads feature columns and anomaly thresholds
- Performs predictions on feature rows
- Calculates severity levels (normal, warning, critical)
- Returns recommendation actions

**Used By:** prediction_api.py, kafka_consumer.py

**Can Remove?** **NO** — Core ML logic, needed for all predictions

---

### **3. db_models.py** ✅ REQUIRED
**Purpose:** SQLAlchemy ORM models for database tables

**Responsibilities:**
- Defines all database table schemas:
  - `ProcessedFeatures` — Time-series feature data
  - `Prediction` — Model predictions
  - `PredictionReview` — Human corrections
  - `RetrainingConfig` — Per-tenant settings
  - `RetrainingRequest` — Retraining workflow
  - `ModelVersion` — Model metadata
  - `ActiveModelVersion` — Currently active model pointer
- Handles database initialization
- Manages database sessions

**Used By:** All service files, prediction_api.py

**Can Remove?** **NO** — Database schema is essential

---

### **4. prediction_service.py** ✅ REQUIRED
**Purpose:** Business logic for managing predictions

**Responsibilities:**
- Save predictions to database (with pending_review status)
- Retrieve predictions by ID, asset, device, status
- Update prediction status (reviewed/archived)
- Count predictions by status/tenant
- Ensure predictions are never saved with user corrections (human-in-the-loop)

**Used By:** prediction_api.py

**Can Remove?** **NO** — Handles all prediction database operations

---

### **5. review_service.py** ✅ REQUIRED
**Purpose:** Business logic for human reviews

**Responsibilities:**
- Store human corrections/reviews
- Mark reviews as training-eligible or ineligible
- Count training-eligible reviews by tenant/label
- Retrieve reviewed predictions with user feedback
- Support for review statistics/analytics

**Used By:** prediction_api.py

**Can Remove?** **NO** — Human-in-the-loop core logic

---

### **6. retraining_service.py** ✅ REQUIRED
**Purpose:** Retraining workflow management

**Responsibilities:**
- Get/set retraining configuration per tenant
- Check retraining eligibility (minimum reviewed records)
- Create retraining requests
- Approve retraining requests with audit trail
- Calculate label distribution (for training data quality)

**Used By:** prediction_api.py

**Can Remove?** **NO** — Required for model improvement workflow

---

### **7. model_version_service.py** ✅ REQUIRED
**Purpose:** Model version tracking and deployment

**Responsibilities:**
- Create model version records
- Track training date, validation score, training data count
- Approve/deploy models
- Manage active model selection
- Retrieve model versions by status

**Used By:** prediction_api.py

**Can Remove?** **NO** — Model versioning is critical for audits

---

### **8. timeseries_repository.py** ✅ REQUIRED
**Purpose:** Abstraction layer for feature storage

**Responsibilities:**
- Save processed features to database
- Retrieve latest features for an asset
- Query features by asset, device, tenant
- Abstraction layer (can be replaced with InfluxDB/TimescaleDB later)

**Used By:** prediction_api.py

**Can Remove?** **NO** — Encapsulates all feature data access

---

### **9. kafka_consumer.py** ⚠️ OPTIONAL
**Purpose:** Kafka-based ML predictions (alternative to REST API)

**Responsibilities:**
- Subscribes to `ml-features` Kafka topic
- Receives preprocessed features from event processing service
- Runs predictions
- Logs results

**Environment Variables:**
```bash
KAFKA_BROKERS=localhost:9092
ML_FEATURES_TOPIC=ml-features
ML_KAFKA_GROUP_ID=ml-service
ML_KAFKA_OFFSET_RESET=latest
```

**Used By:** Optional, for event-driven architecture

**Can Remove?** **YES** — If you're using REST API approach (recommended for now)

**Why Remove?**
- Redundant with FastAPI endpoints
- Adds Kafka dependency complexity
- Your current setup uses REST API (proven working)
- Can be re-added later if event-driven arch is needed

---

## Data Pipeline File (Root Level)

### **10. data_pipeline.py** ⚠️ OPTIONAL
**Purpose:** Full data ingestion and feature computation pipeline

**Responsibilities:**
- Consumes raw sensor data from Kafka `events` topic
- Windowing: buffers N packets before processing
- Noise cancellation: median filtering
- Feature calculation: vibration stats, frequency domain, temperature analysis
- Calls Prediction API (`/predict`)
- Saves results to PostgreSQL

**Environment Variables:**
```bash
KAFKA_BROKER=localhost:9092
KAFKA_TOPIC=events
WINDOW_SIZE=10
DATABASE_URL=postgresql://...
PREDICTION_API_URL=http://localhost:8000/predict/vibration
```

**Used By:** Optional, for full Kafka-to-predictions pipeline

**Can Remove?** **YES** — Partially redundant

**Why Remove?**
- Your ingestion service (`ingestion-service/`) already extracts features
- This duplicates feature calculation logic
- You're using REST API + simulation approach (proven working)
- Can be replaced by `ingestion-service` → Kafka → REST API flow

---

## Test Files (`ai-ml/tests/`)

### **11. test_prediction.py** ⚠️ SMOKE TEST (Can be deprecated)
**Purpose:** Simple smoke test for model loading

**Responsibilities:**
- Loads the ML model
- Tests prediction on one sample row
- Validates model outputs (status, score, severity)

**Used By:** Manual testing, CI/CD verification

**Can Remove?** **YES** — Redundant with integration tests

**Why?** `workflow_integration_test.py` is more comprehensive

---

### **12. test_ml_services.py** ⚠️ UNIT TESTS (Can be deprecated)
**Purpose:** Unit tests for service classes

**Responsibilities:**
- Tests prediction service methods
- Tests review service methods
- Tests database models

**Used By:** Manual unit testing

**Can Remove?** **YES** — Limited maintenance, better coverage exists elsewhere

**Why?** Service tests lack real database integration; integration test is better

---

### **13. workflow_integration_test.py** ✅ KEEP (Most Valuable)
**Purpose:** End-to-end integration test

**Responsibilities:**
- Tests full workflow: features → prediction → review → retraining eligibility
- Validates API responses
- Tests database persistence
- Checks model versioning

**Used By:** Integration testing, CI/CD validation

**Can Remove?** **NO** — This is your best test for validating the entire system

---

### **14. __init__.py** (tests/)
**Purpose:** Python package marker (empty)

**Can Remove?** Not necessary, but harmless

---

### **15. __init__.py** (src/)
**Purpose:** Python package marker (empty)

**Can Remove?** Not necessary, but harmless

---

## Summary Table

| File | Category | Required? | Status | Notes |
|------|----------|-----------|--------|-------|
| `prediction_api.py` | Core API | ✅ YES | In Use | Main service, running daily |
| `prediction_module.py` | ML Logic | ✅ YES | In Use | Model inference, required |
| `db_models.py` | Database | ✅ YES | In Use | Schema definitions |
| `prediction_service.py` | Service | ✅ YES | In Use | Prediction DB operations |
| `review_service.py` | Service | ✅ YES | In Use | Human review workflow |
| `retraining_service.py` | Service | ✅ YES | In Use | Retraining workflow |
| `model_version_service.py` | Service | ✅ YES | In Use | Model versioning |
| `timeseries_repository.py` | Repository | ✅ YES | In Use | Feature data access |
| `kafka_consumer.py` | Optional | ❌ NO | Not Used | Redundant, can remove |
| `data_pipeline.py` | Optional | ❌ NO | Not Used | Overlaps with ingestion-service |
| `test_prediction.py` | Test | ❌ NO | Redundant | Use workflow_integration_test instead |
| `test_ml_services.py` | Test | ❌ NO | Redundant | Use workflow_integration_test instead |
| `workflow_integration_test.py` | Test | ✅ YES | Recommended | Most comprehensive test |
| `__init__.py` (src/) | Package | — | Harmless | Keep for imports |
| `__init__.py` (tests/) | Package | — | Harmless | Keep for imports |

---

## Recommended Cleanup

### **Tier 1: Safe to Remove Immediately** 🗑️

```bash
# Remove optional/redundant files
rm ai-ml/src/kafka_consumer.py          # Kafka consumer (use API instead)
rm ai-ml/data_pipeline.py               # Duplicate feature pipeline
rm ai-ml/tests/test_prediction.py       # Redundant smoke test
rm ai-ml/tests/test_ml_services.py      # Redundant unit tests
```

**Impact:** None — these files aren't used in current setup

**Benefit:** 
- Reduce code maintenance burden
- Simplify deployment
- Clearer codebase structure

---

### **Tier 2: Keep (Production & Testing)** ✅

**Keep all:**
- `ai-ml/src/*.py` (8 files) — Core production code
- `ai-ml/tests/workflow_integration_test.py` — Comprehensive test

**Result:** Clean, focused, maintainable codebase with 9 essential Python files

---

## Migration Path (if needed in future)

If you later want Kafka integration:
1. Keep current REST API approach (proven)
2. Add Kafka consumer *separately* to `event-processing-service/`
3. Don't need `kafka_consumer.py` or `data_pipeline.py`

---

## Quick Decision

**Current Status:** Your setup uses REST API (simulation → API → results)

**Question:** Do you need Kafka consumers?
- **If NO** (likely): Remove files listed in Tier 1
- **If YES** (future): Keep them, but mark as "optional" in README

What would you prefer?
