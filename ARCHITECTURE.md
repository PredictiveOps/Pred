# Human-in-the-Loop ML Pipeline - Complete Architecture Guide

## Overview

This document describes the complete implementation of a human-in-the-loop ML prediction pipeline for predictive maintenance. The system ensures that:

1. **Machine predictions are never used for retraining** without human review
2. **Users can correct predictions** and provide feedback
3. **Retraining is triggered intelligently** based on configured thresholds
4. **All changes are auditable** with user tracking
5. **Model versions are managed** with approval workflows

## System Components

### 1. Python FastAPI Service (`ai-ml/prediction_api.py`)

**Responsibilities:**
- ML model inference
- Processed feature ingestion and storage
- Prediction management
- Review/correction workflows
- Retraining eligibility checking
- Model version management

**Key Endpoints:**
```
POST   /processed-features                    - Save processed features
GET    /processed-features/latest/{asset_id} - Get latest features
POST   /predict                               - Run prediction
GET    /predictions/pending                   - Get pending predictions
POST   /predictions/{id}/review               - Submit human review
GET    /reviews                               - Get all reviews
GET    /retraining/eligibility                - Check retraining eligibility
POST   /retraining/request                    - Create retraining request
POST   /retraining/{id}/approve               - Approve retraining
GET    /models/versions                       - Get model versions
POST   /models/{id}/{version}/approve         - Approve model version
POST   /models/{id}/{version}/deploy          - Deploy model version
```

### 2. Go Event Processing Service (`event-processing-service/`)

**Database Models:**
- `Event` - Raw sensor events (existing)
- `ProcessedFeatures` - Time-series feature data
- `Prediction` - ML predictions (pending_review status)
- `PredictionReview` - Human-reviewed predictions
- `RetrainingConfig` - Per-tenant retraining configuration
- `RetrainingRequest` - Retraining workflow state
- `ModelVersion` - Model metadata and versions
- `ActiveModelVersion` - Current active model pointer

**Admin API Routes:**
```
GET    /tenants/:tenantID/predictions/pending          - Pending predictions
GET    /tenants/:tenantID/reviews                      - All reviews
GET    /tenants/:tenantID/reviews/training-eligible-count
GET    /tenants/:tenantID/retraining/config            - Config
PUT    /tenants/:tenantID/retraining/config            - Update config
POST   /tenants/:tenantID/retraining/:requestID/approve
GET    /tenants/:tenantID/models/versions              - Model versions
GET    /tenants/:tenantID/models/active                - Active model
POST   /tenants/:tenantID/models/:modelID/:version/approve
POST   /tenants/:tenantID/models/:modelID/:version/deploy
```

### 3. Python Service Layer

#### TimeSeriesRepository
```python
- save_processed_features()
- get_latest_features()
- get_features_by_asset()
- get_features_by_device()
```
**Abstraction for easy InfluxDB/TimescaleDB migration**

#### PredictionService
```python
- save_prediction()              # Save as pending_review
- get_prediction_by_id()
- get_pending_predictions()
- get_reviewed_predictions()
- update_prediction_status()
- get_predictions_by_asset()
- count_predictions_by_status()
```

#### ReviewService
```python
- review_prediction()            # Store human correction
- get_reviewed_predictions()
- get_training_eligible_reviews()
- count_training_eligible_reviews()
- count_reviews_by_label()
- mark_training_ineligible()
- get_reviews_by_asset()
```

#### RetrainingService
```python
- get_config()
- set_config()
- check_retraining_eligibility() # Checks thresholds + label distribution
- create_retraining_request()
- approve_retraining_request()
- reject_retraining_request()
- update_request_status()
```

#### ModelVersionService
```python
- create_model_version()
- get_model_version()
- get_latest_model_version()
- get_active_model()
- approve_model_version()
- deploy_model_version()         # Sets as active
- reject_model_version()
- get_model_versions_by_status()
```

## Data Flow

### Prediction Flow

```
1. Sensor Data → Ingestion Service (MQTT/HTTP)
                     ↓
2. Process Features → Post to /processed-features
                     ↓
3. Store in ProcessedFeatures table (time-series)
                     ↓
4. Call /predict endpoint
                     ↓
5. Load latest features from ProcessedFeatures
                     ↓
6. Run ML model inference
                     ↓
7. Save Prediction with status=pending_review
                     ↓
8. Return prediction_id to dashboard
```

### Review Flow

```
1. Dashboard calls /predictions/pending
                     ↓
2. User reviews prediction (sees model output + features)
                     ↓
3. User submits review with:
   - reviewed_label (corrected classification)
   - review_comment (optional)
   - is_training_eligible (flag for retraining)
                     ↓
4. POST /predictions/{id}/review stores:
   - PredictionReview record
   - Updates Prediction status to "reviewed"
                     ↓
5. System checks if retraining is eligible:
   - Count training-eligible reviews
   - Compare against threshold
   - Check label distribution (need normal, warning, critical)
```

### Retraining Flow

```
1. System or user calls GET /retraining/eligibility
                     ↓
2. Service checks:
   - training_eligible_count >= minimum_reviewed_records
   - Have samples of all label types (normal, warning, critical)
                     ↓
3. If eligible:
   - POST /retraining/request creates RetrainingRequest
   - Status = "created"
                     ↓
4. Authorized user approves:
   - POST /retraining/{id}/approve
   - Status = "approved"
   - approved_by = user_id
                     ↓
5. External retraining job:
   - Listens for approved requests
   - Queries all training_eligible reviews
   - Trains new model
   - Returns validation metrics
   - Creates new ModelVersion
   - Status = "trained"
                     ↓
6. User reviews model metrics and approves:
   - POST /models/{id}/{version}/approve
   - Status = "approved"
                     ↓
7. User deploys:
   - POST /models/{id}/{version}/deploy
   - Status = "deployed"
   - Updates ActiveModelVersion pointer
   - Future predictions use this version
```

## Multi-Tenancy

**All operations are tenant-scoped:**

```python
# Python API
@app.post("/predict")
def predict(request: PredictRequest, db: Session):
    # request.tenant_id is required
    # All queries filtered by tenant_id
    services["prediction"].get_prediction_by_id(
        tenant_id=request.tenant_id,  # ← Required
        prediction_id=prediction_id
    )
```

```go
// Go API
func getPendingPredictions(gdb *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.Param("tenantID")  // ← From URL
        // All queries filtered by tenantID
        db.GetPendingPredictions(ctx, gdb, tenantID, limit)
    }
}
```

**Each tenant has:**
- Separate ProcessedFeatures data
- Separate Predictions and Reviews
- Own RetrainingConfig
- Own ModelVersion history
- Own ActiveModelVersion pointer

## Configuration

### Retraining Config Per Tenant

```json
{
  "minimum_reviewed_records": 500,     // Threshold for retraining
  "auto_retrain_enabled": false,       // Auto-create requests when eligible
  "requires_manual_approval": true     // Need approval before training
}
```

### Model Deployment States

```
trained          → Model trained, ready for review
pending_approval → Awaiting human review/approval
approved         → Approved, ready to deploy
deployed         → Currently active, used for predictions
rejected         → Approval denied, not used
```

### Prediction Review Status

```
pending_review  → Awaiting human review
reviewed        → Reviewed by human
archived        → Old/historical
```

## Key Features

### 1. Intelligent Retraining

**Eligibility checks:**
- ✓ Sufficient reviewed records (≥ threshold)
- ✓ Label distribution: must have samples of normal, warning, critical
- ✓ Only training_eligible=true reviews are counted
- ✓ Never uses unreviewed model predictions

**Approval workflow:**
- Optional auto-trigger based on thresholds
- Authorized user approval required (if enabled)
- Audit trail: who requested, who approved

### 2. Human-in-the-Loop Review

**Each prediction review captures:**
- Model's original prediction
- User's corrected label (if different)
- Comment from reviewer
- Training eligibility decision
- Timestamp and reviewer ID

**Dashboard shows:**
- Pending predictions needing review
- Features that triggered prediction
- Model confidence/score
- History of predictions for asset

### 3. Model Versioning

**Complete version history:**
- Training date, data count, validation score
- Approval status and approver
- Deployment status (active/inactive)
- Rollback capability via active model pointer

**Model deployment:**
- Only approved versions can be deployed
- Active version pointer updated atomically
- Previous version kept for rollback
- Clear audit trail

### 4. Repository Pattern

**TimeSeriesRepository:**
```python
# Easy to swap implementations
repo = TimeSeriesRepository(db_session)  # PostgreSQL
# Later: repo = InfluxDBRepository(...)
# API code remains unchanged
```

## Integration Guide

### 1. Event Processing Service Integration

The Go event-processing service now has routes to access predictions and reviews:

```go
// Get pending predictions for dashboard
GET /tenants/tenant_123/predictions/pending

// Get retraining eligibility
GET /tenants/tenant_123/retraining/eligibility

// Approve retraining request
POST /tenants/tenant_123/retraining/req_id/approve
```

### 2. Web Frontend Integration

```typescript
// Get pending predictions
const { data: predictions } = await fetch(
  '/api/predictions/pending?tenant_id=tenant_123'
);

// Review a prediction
const { data: review } = await fetch(
  `/api/predictions/${predictionId}/review`,
  {
    method: 'POST',
    body: JSON.stringify({
      reviewed_label: 'warning',
      reviewed_by: userId,
      is_training_eligible: true,
    })
  }
);

// Check retraining eligibility
const { data: eligibility } = await fetch(
  '/api/retraining/eligibility?tenant_id=tenant_123'
);
```

### 3. Retraining Pipeline Integration

**External retraining job:**

```python
# 1. Check for approved requests
requests = db.query(
    "SELECT * FROM retraining_requests WHERE status='approved'"
)

# 2. For each approved request
for req in requests:
    # 3. Get training data
    reviews = db.query(
        "SELECT * FROM prediction_reviews WHERE "
        "tenant_id=? AND is_training_eligible=true",
        req.tenant_id
    )
    
    # 4. Train model
    new_model = train_model(reviews)
    
    # 5. Create model version
    POST /models/create
        {
            "model_id": "vibration_isolation_forest_v2",
            "training_data_count": len(reviews),
            "validation_score": 0.95,
            "model_path": "s3://models/vibration_v2.pkl"
        }
    
    # 6. Update request status
    UPDATE retraining_requests SET status='completed'
```

## Testing

Run integration tests:

```bash
cd ai-ml
python workflow_integration_test.py
```

This runs the complete workflow:
1. Save features
2. Run prediction
3. Review prediction
4. Check retraining eligibility
5. Create retraining request
6. Approve retraining request
7. Check model versions

## Extensibility Points

### 1. Storage Backend

Replace PostgreSQL with InfluxDB/TimescaleDB:

```python
# Current
class TimeSeriesRepository:
    def __init__(self, session: Session):
        # SQLAlchemy session

# Future
class InfluxDBRepository:
    def __init__(self, client: InfluxDBClient):
        # InfluxDB client
    
    # Same interface
    def save_processed_features(self, ...):
        # InfluxDB write()
```

### 2. Retraining Implementation

Customize retraining logic:

```python
class CustomRetrainingService(RetrainingService):
    def check_retraining_eligibility(self, tenant_id):
        # Custom logic: more/fewer records needed
        # Different label distribution requirements
        # Custom scheduling logic
        pass
```

### 3. Model Management

Integrate with MLOps platform:

```python
# Current: stores in db
model_service.create_model_version(...)

# Future: also register with
# - MLflow Model Registry
# - Seldon Core
# - BentoML
# - KServe
```

## Deployment

### Prerequisites

- PostgreSQL 12+ (shared with other services)
- Python 3.9+
- Dependencies: see requirements.txt

### Steps

1. **Update database:**
   ```bash
   # Go service will auto-migrate on startup
   cd event-processing-service
   go run .
   ```

2. **Install Python deps:**
   ```bash
   cd ai-ml
   pip install -r requirements.txt
   ```

3. **Configure environment:**
   ```bash
   cp .env.example .env
   # Edit DATABASE_URL
   ```

4. **Start API:**
   ```bash
   uvicorn prediction_api:app --host 0.0.0.0 --port 8000
   ```

5. **Verify setup:**
   ```bash
   curl http://localhost:8000/health
   curl http://localhost:8080/health
   ```

## Troubleshooting

### Issue: "No features found"

**Cause:** No ProcessedFeatures saved for asset yet
**Solution:** POST to `/processed-features` first

### Issue: "Retraining not eligible"

**Check:**
```bash
curl "http://localhost:8000/retraining/eligibility?tenant_id=TENANT"
```

**Possible causes:**
- Insufficient reviews (< threshold)
- Missing label types (need normal, warning, critical)
- Reviews marked as training_ineligible

### Issue: Model not found

**Check:**
```bash
curl "http://localhost:8000/models/active?tenant_id=TENANT"
```

**Solution:** Deploy a model version first

## Future Enhancements

1. **Automated Retraining** - Scheduled or event-driven
2. **Model Monitoring** - Track accuracy on reviews over time
3. **Drift Detection** - Detect data/concept drift
4. **A/B Testing** - Deploy multiple models, compare performance
5. **Explainability** - SHAP values, feature importance
6. **Batch Inference** - Process multiple features at once
7. **Performance Metrics** - Model latency, throughput
8. **Alert Thresholds** - Recalibrate based on business rules
