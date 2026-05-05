# Quick Start Guide - Human-in-the-Loop ML Pipeline

## What Was Implemented

A complete human-in-the-loop machine learning pipeline for predictive maintenance that ensures:
- ✅ Processed features stored in time-series database
- ✅ ML predictions saved as pending_review
- ✅ Human users review and correct predictions
- ✅ Intelligent retraining triggers based on thresholds
- ✅ Model versioning with approval workflows
- ✅ Complete audit trail

## Files Created/Modified

### Python Service (`ai-ml/`)

| File | Purpose |
|------|---------|
| `db_models.py` | SQLAlchemy models for pipeline (ProcessedFeatures, Prediction, Review, RetrainingConfig, ModelVersion) |
| `timeseries_repository.py` | Abstraction layer for time-series feature storage (easy InfluxDB/TimescaleDB migration) |
| `prediction_service.py` | Service for managing predictions |
| `review_service.py` | Service for human reviews and corrections |
| `retraining_service.py` | Orchestrates retraining workflow |
| `model_version_service.py` | Manages model versioning and deployment |
| `prediction_api.py` | FastAPI endpoints (replaced entire file) |
| `requirements.txt` | Updated with SQLAlchemy, psycopg2 |
| `PIPELINE_README.md` | Complete pipeline documentation |
| `.env.example` | Environment configuration template |
| `workflow_integration_test.py` | Integration test examples |

### Go Service (`event-processing-service/`)

| File | Purpose |
|------|---------|
| `db/models.go` | New database models (ProcessedFeatures, Prediction, Review, etc.) |
| `db/queries.go` | CRUD operations for new models |
| `db/db.go` | Auto-migration for new models |
| `api/prediction_routes.go` | Admin routes for predictions and reviews |
| `api/router.go` | Routes registration |

### Documentation

| File | Purpose |
|------|---------|
| `ARCHITECTURE.md` | Complete architecture and integration guide |

## Database Schema

All tables automatically created on startup via auto-migration.

**Time-Series Storage:**
```
processed_features
  - tenant_id, device_id, asset_id
  - features (JSON), feature_version
  - feature_timestamp, created_at
```

**Predictions & Reviews:**
```
predictions
  - prediction_id, tenant_id, device_id, asset_id
  - model_name, model_version, anomaly_score
  - predicted_status, review_status, reviewed

prediction_reviews
  - review_id, prediction_id, tenant_id
  - model_prediction, reviewed_label
  - reviewed_by, review_comment, is_training_eligible
```

**Retraining:**
```
retraining_configs
  - minimum_reviewed_records, auto_retrain_enabled
  - requires_manual_approval

retraining_requests
  - request_id, status (created/approved/in_progress/completed)
  - training_data_count, requested_by, approved_by
```

**Model Management:**
```
model_versions
  - model_id, model_name, model_version
  - training_data_count, validation_score
  - deployment_status (trained/approved/deployed/rejected)

active_model_versions
  - tenant_id -> current active model pointer
```

## API Endpoints

### Python Service (Port 8000)

**Processed Features:**
- `POST /processed-features` - Save features
- `GET /processed-features/latest/{asset_id}?tenant_id=X` - Get latest

**Predictions:**
- `POST /predict` - Run prediction (saves as pending_review)
- `GET /predictions/pending?tenant_id=X` - Get pending
- `GET /predictions/{id}?tenant_id=X` - Get specific

**Reviews:**
- `POST /predictions/{id}/review?tenant_id=X` - Submit review
- `GET /reviews?tenant_id=X` - Get all reviews
- `GET /reviews/training-eligible-count?tenant_id=X` - Count eligible

**Retraining:**
- `GET /retraining/config?tenant_id=X` - Get config
- `PUT /retraining/config?tenant_id=X` - Set config
- `GET /retraining/eligibility?tenant_id=X` - Check eligibility
- `POST /retraining/request?tenant_id=X&requested_by=USER` - Create request
- `POST /retraining/{id}/approve?tenant_id=X&approved_by=USER` - Approve

**Models:**
- `GET /models/versions?tenant_id=X` - Get versions
- `POST /models/{id}/{version}/approve?tenant_id=X` - Approve
- `POST /models/{id}/{version}/deploy?tenant_id=X` - Deploy
- `GET /models/active?tenant_id=X` - Get active

### Go Service (Port 8080) - Admin Routes

- `GET /tenants/:tenantID/predictions/pending` - Pending predictions
- `GET /tenants/:tenantID/reviews` - Reviews
- `GET /tenants/:tenantID/retraining/config` - Config
- `PUT /tenants/:tenantID/retraining/config` - Update config
- `POST /tenants/:tenantID/models/:modelID/:version/deploy` - Deploy

## Setup Instructions

### 1. Install Dependencies

```bash
cd ai-ml
pip install -r requirements.txt
```

Adds:
- `sqlalchemy>=2.0` - ORM
- `psycopg2-binary` - PostgreSQL driver
- `python-dotenv` - Configuration
- Other: pandas, scikit-learn, pydantic

### 2. Configure Database

```bash
cd ai-ml
cp .env.example .env

# Edit .env
# DATABASE_URL=postgresql://user:password@localhost:5433/predictions
```

### 3. Run Python Service

```bash
cd ai-ml
uvicorn prediction_api:app --host 0.0.0.0 --port 8000 --reload
```

Visit: `http://localhost:8000/docs` for interactive API documentation

### 4. Run Go Service

```bash
cd event-processing-service
go run .
```

Tables auto-created on startup.

## Usage Examples

### Example 1: Save Features and Run Prediction

```bash
# 1. Save processed features
curl -X POST http://localhost:8000/processed-features \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant_1",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "features": {
      "rms": 2.5,
      "kurtosis": 3.8,
      "temperature": 45.3
    },
    "feature_timestamp": "2024-05-02T10:00:00Z"
  }'

# 2. Run prediction
curl -X POST http://localhost:8000/predict \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant_1",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "features": {
      "rms": 2.5,
      "kurtosis": 3.8,
      "temperature": 45.3
    }
  }'
```

### Example 2: Review Prediction

```bash
# Get pending predictions
curl "http://localhost:8000/predictions/pending?tenant_id=tenant_1"

# Submit review (correct the prediction)
curl -X POST "http://localhost:8000/predictions/PRED_ID/review?tenant_id=tenant_1" \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "reviewed_label": "warning",
    "reviewed_by": "engineer_1",
    "review_comment": "Model predicted critical, but vibration trend shows warning",
    "is_training_eligible": true
  }'
```

### Example 3: Retraining Workflow

```bash
# 1. Check retraining eligibility
curl "http://localhost:8000/retraining/eligibility?tenant_id=tenant_1"

# 2. Create retraining request
curl -X POST "http://localhost:8000/retraining/request?tenant_id=tenant_1&requested_by=engineer_1"

# 3. Approve retraining
curl -X POST "http://localhost:8000/retraining/REQ_ID/approve?tenant_id=tenant_1&approved_by=admin_1"

# 4. (External job trains model and creates ModelVersion)

# 5. Approve model version
curl -X POST "http://localhost:8000/models/MODEL_ID/v2/approve?tenant_id=tenant_1&approved_by=admin_1"

# 6. Deploy model
curl -X POST "http://localhost:8000/models/MODEL_ID/v2/deploy?tenant_id=tenant_1"
```

## Integration Points

### Event Processing Service

Now exposes admin routes for predictions and reviews:

```go
// In your Go code
GET /tenants/tenant_1/predictions/pending
GET /tenants/tenant_1/reviews
GET /tenants/tenant_1/retraining/eligibility
```

### Web Frontend

Can integrate review dashboard:

```typescript
// Get pending predictions
const predictions = await fetch('/api/predictions/pending?tenant_id=tenant_1');

// Submit review
await fetch('/api/predictions/{id}/review?tenant_id=tenant_1', {
  method: 'POST',
  body: JSON.stringify({
    reviewed_label: 'warning',
    reviewed_by: userId,
    is_training_eligible: true,
  })
});
```

### External Retraining Pipeline

Listen for approved requests and:

```python
# 1. Get approved retraining requests
requests = db.query("SELECT * FROM retraining_requests WHERE status='approved'")

# 2. For each request, get training data
reviews = db.query(
    "SELECT * FROM prediction_reviews WHERE is_training_eligible=true"
)

# 3. Train new model

# 4. Create model version via API
POST /models/create { ... }

# 5. Update request status to completed
```

## Configuration

### Retraining Config

```bash
curl -X PUT "http://localhost:8000/retraining/config?tenant_id=tenant_1&updated_by=admin_1" \
  -H "Content-Type: application/json" \
  -d '{
    "minimum_reviewed_records": 500,
    "auto_retrain_enabled": false,
    "requires_manual_approval": true
  }'
```

**Options:**
- `minimum_reviewed_records` - Number of eligible reviews needed (default: 500)
- `auto_retrain_enabled` - Automatically create requests when threshold met (default: false)
- `requires_manual_approval` - Need approval before retraining (default: true)

## Testing

Run integration test:

```bash
cd ai-ml
python workflow_integration_test.py
```

This runs the complete workflow:
1. Health check
2. Save features
3. Run prediction
4. Get pending predictions
5. Review prediction
6. Check retraining eligibility
7. Create retraining request
8. Approve retraining request

## Key Concepts

### Review Status vs Reviewed Flag

- **review_status**: workflow state (pending_review, reviewed, archived)
- **reviewed**: boolean flag indicating human has reviewed

### Training Eligibility

Only reviews marked with `is_training_eligible=true` are counted for retraining.

Use case: Mark problematic reviews as ineligible if they contain annotation errors.

### Model Deployment Status

1. `trained` - Model trained, ready for review
2. `pending_approval` - Awaiting human decision
3. `approved` - Approved, can be deployed
4. `deployed` - Currently active model
5. `rejected` - Approval denied

### Multi-Tenancy

Every operation requires `tenant_id`:
- Predictions are tenant-scoped
- Reviews are tenant-scoped
- Each tenant has own retraining config
- Each tenant has own active model

## Troubleshooting

### "No features found"

Save features first:
```bash
POST /processed-features
```

### "Retraining not eligible"

Check eligibility details:
```bash
GET /retraining/eligibility?tenant_id=X
```

Possible issues:
- Too few reviews (< threshold)
- Missing label distribution
- Reviews marked as ineligible

### Model not deploying

Check model version status:
```bash
GET /models/versions?tenant_id=X&status=trained
```

Must be `trained` before you can `approve`, then `deploy`.

## Next Steps

1. **Integrate with dashboard** - Add review UI for pending predictions
2. **Implement retraining pipeline** - Listen for approved requests and retrain
3. **Add model monitoring** - Track accuracy on reviews over time
4. **Enable auto-retraining** - Schedule automatic retraining jobs
5. **Migrate to InfluxDB** - Replace TimeSeriesRepository implementation

## Support

For architecture questions, see: [ARCHITECTURE.md](./ARCHITECTURE.md)

For API documentation, see: [ai-ml/PIPELINE_README.md](./ai-ml/PIPELINE_README.md)

For code examples, see: [ai-ml/workflow_integration_test.py](./ai-ml/workflow_integration_test.py)
