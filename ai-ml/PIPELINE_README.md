# Bearing Anomaly Prediction API - Human-in-the-Loop ML Pipeline

## Overview

This is a comprehensive ML prediction pipeline implementation for bearing/vibration predictive maintenance with **human-in-the-loop** review and retraining workflow.

### Key Features

1. **Time-Series Feature Storage** - Processed sensor features stored in PostgreSQL with time-series indexes
2. **ML Predictions with Review Status** - Predictions saved as pending_review for human validation
3. **Human Review Workflow** - Authorized users can review, correct, and mark predictions for retraining
4. **Intelligent Retraining Triggers** - System checks data thresholds and label distribution before retraining
5. **Model Versioning** - Track all model versions with training metadata and deployment status
6. **Approval Workflows** - Optional manual approval for retraining and model deployment

## Architecture

### Data Flow

```
Processed Features → Time-Series DB → ML Model → Prediction (pending_review)
                                                        ↓
                                                   Human Review
                                                        ↓
                                              Review Stored + Eligible?
                                                        ↓
                                        Enough eligible reviews? (≥500 default)
                                                        ↓
                                        Create Retraining Request
                                                        ↓
                                        Authorized User Approves
                                                        ↓
                                        Retrain Model with Reviewed Data
                                                        ↓
                                        Create New Model Version
                                                        ↓
                                        Authorized User Approves Deployment
                                                        ↓
                                        Deploy New Model Version (Active)
```

### Database Models

#### Time-Series Storage
- **ProcessedFeatures** - Raw processed sensor features for ML consumption

#### Predictions & Reviews
- **Prediction** - ML model output before review
- **PredictionReview** - Human-reviewed predictions with corrections

#### Retraining Workflow
- **RetrainingConfig** - Configuration per tenant (minimum_records, auto_retrain, approval_required)
- **RetrainingRequest** - Retraining workflow state (created → approved → in_progress → completed)

#### Model Management
- **ModelVersion** - Versioned model metadata and deployment status
- **ActiveModelVersion** - Pointer to currently active model per tenant

## API Endpoints

### Health Check
- `GET /health` - Service health status

### Processed Features
- `POST /processed-features` - Save processed features
- `GET /processed-features/latest/{asset_id}?tenant_id=X` - Get latest features for asset

### Predictions
- `POST /predict` - Run ML prediction on latest features (saves as pending_review)
- `GET /predictions/pending?tenant_id=X` - Get predictions awaiting review
- `GET /predictions/{prediction_id}?tenant_id=X` - Get specific prediction

### Reviews
- `POST /predictions/{prediction_id}/review?tenant_id=X` - Submit human review/correction
- `GET /reviews?tenant_id=X` - Get all reviews
- `GET /reviews/training-eligible-count?tenant_id=X` - Count eligible reviews

### Retraining Configuration
- `GET /retraining/config?tenant_id=X` - Get configuration
- `PUT /retraining/config?tenant_id=X&updated_by=USER` - Set/update configuration
- `GET /retraining/eligibility?tenant_id=X` - Check retraining eligibility

### Retraining Workflow
- `POST /retraining/request?tenant_id=X&requested_by=USER` - Create retraining request
- `POST /retraining/{request_id}/approve?tenant_id=X&approved_by=USER` - Approve retraining

### Model Management
- `GET /models/versions?tenant_id=X&status=DEPLOYED` - Get model versions
- `POST /models/{model_id}/{version}/approve?tenant_id=X&approved_by=USER` - Approve version
- `POST /models/{model_id}/{version}/deploy?tenant_id=X` - Deploy version (set active)
- `GET /models/active?tenant_id=X` - Get currently active model

## Setup

### 1. Install Dependencies

```bash
cd ai-ml
pip install -r requirements.txt
```

### 2. Configure Environment

```bash
cp .env.example .env
# Edit .env with your database URL
```

### 3. Initialize Database

The API will auto-create all tables on startup via SQLAlchemy.

### 4. Run API

```bash
uvicorn prediction_api:app --host 0.0.0.0 --port 8000 --reload
```

API docs available at: `http://localhost:8000/docs`

## Usage Examples

### 1. Save Processed Features

```bash
curl -X POST http://localhost:8000/processed-features \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant_123",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "features": {
      "rms": 2.5,
      "kurtosis": 3.8,
      "crest_factor": 4.2,
      "spectral_energy": 156.7,
      "temperature": 45.3
    },
    "feature_timestamp": "2024-05-02T10:30:00Z"
  }'
```

### 2. Run Prediction

```bash
curl -X POST http://localhost:8000/predict \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant_123",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "features": {
      "rms": 2.5,
      "kurtosis": 3.8,
      "crest_factor": 4.2,
      "spectral_energy": 156.7,
      "temperature": 45.3
    }
  }'
```

### 3. Get Pending Predictions

```bash
curl "http://localhost:8000/predictions/pending?tenant_id=tenant_123&limit=10"
```

### 4. Review a Prediction

```bash
curl -X POST http://localhost:8000/predictions/pred_xxx/review?tenant_id=tenant_123 \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "reviewed_label": "warning",
    "reviewed_by": "engineer_001",
    "review_comment": "High vibration but no immediate failure risk",
    "is_training_eligible": true
  }'
```

### 5. Check Retraining Eligibility

```bash
curl "http://localhost:8000/retraining/eligibility?tenant_id=tenant_123"
```

### 6. Create Retraining Request

```bash
curl -X POST "http://localhost:8000/retraining/request?tenant_id=tenant_123&requested_by=engineer_001"
```

### 7. Approve Retraining

```bash
curl -X POST "http://localhost:8000/retraining/req_xxx/approve?tenant_id=tenant_123&approved_by=admin_001"
```

## Services Architecture

### TimeSeriesRepository
Manages processed feature data with abstraction layer for easy InfluxDB/TimescaleDB migration.

### PredictionService
Handles ML prediction storage and retrieval.

### ReviewService
Manages human reviews with training eligibility filtering.

### RetrainingService
Orchestrates retraining workflow:
- Checks eligibility based on configured thresholds
- Creates retraining requests
- Tracks approval workflow
- Validates label distribution

### ModelVersionService
Manages model versioning:
- Creates new versions
- Tracks deployment status (trained → pending_approval → approved → deployed)
- Manages active model pointer
- Supports rollback via active model selection

## Key Design Principles

1. **Separation of Concerns**
   - Time-series storage separate from predictions
   - Predictions separate from reviews
   - Unreviewed predictions never used for retraining

2. **Multi-Tenancy**
   - All queries filtered by tenant_id
   - Isolated configurations per tenant
   - Separate active model per tenant

3. **Approval Workflows**
   - Optional automatic triggers
   - Manual approval for critical decisions
   - Audit trails (reviewed_by, approved_by fields)

4. **Extensibility**
   - Repository pattern for easy storage backend swaps
   - Service layer abstracts business logic
   - Clear interfaces for integration with retraining pipelines

## Integration Points

### Existing Services
- **Event Processing Service**: Can ingest processed features via POST /processed-features
- **Web Frontend**: Can call review endpoints with user context
- **Dashboard**: Can query pending predictions and reviews

### Future Integrations
- **Retraining Pipeline**: Listen for approved retraining requests, execute training, create new ModelVersion
- **Monitoring**: Subscribe to model deployment events
- **InfluxDB/TimescaleDB**: Implement TimeSeriesRepository interface

## Configuration

### Retraining Config

```json
{
  "minimum_reviewed_records": 500,
  "auto_retrain_enabled": false,
  "requires_manual_approval": true
}
```

- **minimum_reviewed_records**: Number of eligible reviews needed before retraining
- **auto_retrain_enabled**: If true, automatically create retraining requests when threshold met
- **requires_manual_approval**: If true, require authorized user approval before retraining starts

### Model Deployment Status Values

- `trained` - Model trained, ready for review
- `pending_approval` - Awaiting human approval
- `approved` - Approved, ready for deployment
- `deployed` - Currently active model
- `rejected` - Approval declined

## Testing

See [test_prediction.py](./test_prediction.py) for integration test examples.

## Troubleshooting

### Database Connection Issues
- Verify DATABASE_URL in .env
- Check PostgreSQL is running and accessible
- Verify credentials and permissions

### Model Loading Issues
- Check model files exist in results/models/
- Verify feature column names match
- Ensure threshold files are properly formatted

### Retraining Ineligibility
- Check `/retraining/eligibility` endpoint
- Ensure sufficient reviewed records (default: 500)
- Verify label distribution includes normal, warning, critical

## Future Enhancements

1. **Auto-Retraining Pipeline** - Service that listens for approved requests and executes training
2. **A/B Testing** - Deployment of multiple models, traffic routing
3. **Drift Detection** - Monitor for data/concept drift
4. **Explainability** - Feature importance, SHAP values
5. **Performance Tracking** - Monitor model accuracy on reviews over time
6. **Batch Processing** - Retraining on scheduled basis
7. **Model Registry** - Centralized model artifact storage
