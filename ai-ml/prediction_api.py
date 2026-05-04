"""
Bearing Anomaly Prediction API with Human-in-the-Loop ML Pipeline

This API provides:
1. Processed feature data ingestion
2. ML model predictions with pending review
3. Human review/correction workflow
4. Retraining eligibility checking
5. Model versioning and deployment management
6. Dashboard support for authorized users
"""

from __future__ import annotations

import os
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional

from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException, Query
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from db_models import (
    get_db_engine,
    get_session,
    init_db,
)
from model_version_service import ModelVersionService
from prediction_module import BearingAnomalyPredictor
from prediction_service import PredictionService
from review_service import ReviewService
from retraining_service import RetrainingService
from timeseries_repository import TimeSeriesRepository

# Load environment
load_dotenv()

# Configuration
PROJECT_ROOT = Path(__file__).resolve().parent
MODEL_DIR = PROJECT_ROOT / "results" / "models"
DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://user:password@localhost:5433/predictions",
)

# Initialize database
engine = init_db(DATABASE_URL)

# Load ML model
PREDICTOR = BearingAnomalyPredictor(
    model_path=MODEL_DIR / "vibration_isolation_forest.pkl",
    feature_columns_path=MODEL_DIR / "feature_columns.json",
    thresholds_path=MODEL_DIR / "anomaly_thresholds.json",
)

# FastAPI app
app = FastAPI(
    title="Bearing Anomaly Prediction API",
    description="Human-in-the-loop ML pipeline for predictive maintenance",
    version="1.0.0",
)


# ============================================================================
# Pydantic Models for API Requests/Responses
# ============================================================================


class ProcessedFeaturesRequest(BaseModel):
    tenant_id: str = Field(..., description="Tenant identifier")
    device_id: str = Field(..., description="Device identifier")
    asset_id: str = Field(..., description="Asset identifier")
    features: dict[str, Any] = Field(..., description="Processed features (rms, kurtosis, etc.)")
    feature_timestamp: datetime = Field(..., description="When features were collected")
    feature_version: str = Field(default="v1", description="Feature schema version")


class ProcessedFeaturesResponse(BaseModel):
    id: int
    tenant_id: str
    device_id: str
    asset_id: str
    features: dict
    feature_version: str
    created_at: datetime
    feature_timestamp: datetime


class PredictRequest(BaseModel):
    tenant_id: str = Field(..., description="Tenant identifier")
    device_id: str = Field(..., description="Device identifier")
    asset_id: str = Field(..., description="Asset identifier")
    features: dict[str, Any] = Field(..., description="Feature values for prediction")


class PredictionResponse(BaseModel):
    prediction_id: str
    tenant_id: str
    device_id: str
    asset_id: str
    model_name: str
    model_version: str
    anomaly_score: float
    predicted_status: str
    review_status: str
    reviewed: bool
    timestamp: datetime


class PredictionDetailResponse(PredictionResponse):
    severity_level: int
    is_anomaly: bool
    recommended_action: str


class ReviewPredictionRequest(BaseModel):
    device_id: str = Field(..., description="Device identifier")
    asset_id: str = Field(..., description="Asset identifier")
    reviewed_label: str = Field(..., description="Corrected label (normal, warning, critical)")
    reviewed_by: str = Field(..., description="User ID of reviewer")
    review_comment: Optional[str] = Field(None, description="Optional comment")
    is_training_eligible: bool = Field(default=True, description="Mark for retraining")


class PredictionReviewResponse(BaseModel):
    review_id: str
    prediction_id: str
    tenant_id: str
    device_id: str
    asset_id: str
    model_prediction: str
    reviewed_label: str
    reviewed_by: str
    review_comment: Optional[str]
    is_training_eligible: bool
    reviewed_at: datetime


class RetrainingConfigRequest(BaseModel):
    minimum_reviewed_records: int = Field(default=500, description="Minimum reviews needed")
    auto_retrain_enabled: bool = Field(default=False, description="Auto-trigger retraining")
    requires_manual_approval: bool = Field(default=True, description="Need approval to retrain")


class RetrainingConfigResponse(BaseModel):
    tenant_id: str
    minimum_reviewed_records: int
    auto_retrain_enabled: bool
    requires_manual_approval: bool
    updated_by: Optional[str]
    updated_at: datetime


class RetrainingEligibilityResponse(BaseModel):
    eligible: bool
    eligible_count: int
    required_count: int
    reason: Optional[str] = None
    label_distribution: dict[str, int]
    missing_labels: Optional[list[str]] = None


class RetrainingRequestResponse(BaseModel):
    request_id: str
    tenant_id: str
    status: str
    training_data_count: int
    requested_by: str
    approved_by: Optional[str]
    created_at: datetime
    updated_at: datetime


class ApproveRetrainingRequest(BaseModel):
    approved_by: str = Field(..., description="User ID approving")


class ModelVersionResponse(BaseModel):
    model_id: str
    model_name: str
    model_version: str
    deployment_status: str
    training_data_count: Optional[int]
    training_date: Optional[datetime]
    validation_score: Optional[float]
    approved_by: Optional[str]
    created_at: datetime


class ApproveModelRequest(BaseModel):
    approved_by: str = Field(..., description="User ID approving")


class HealthResponse(BaseModel):
    status: str
    service: str
    version: str


# ============================================================================
# Dependency Functions
# ============================================================================


def get_db() -> Session:
    """Get database session."""
    return get_session(engine)


def get_services(db: Session):
    """Factory for services. Reuses ReviewService instance for efficiency."""
    review_service = ReviewService(db)
    return {
        "timeseries": TimeSeriesRepository(db),
        "prediction": PredictionService(db),
        "review": review_service,
        "retraining": RetrainingService(db, review_service),
        "model_version": ModelVersionService(db),
    }


# ============================================================================
# Health Check Endpoints
# ============================================================================


@app.get("/health", response_model=HealthResponse)
def health():
    """Health check endpoint."""
    return {
        "status": "ok",
        "service": "bearing_anomaly_prediction",
        "version": "1.0.0",
    }


# ============================================================================
# Processed Features Endpoints
# ============================================================================


@app.post("/processed-features", response_model=ProcessedFeaturesResponse)
def save_processed_features(request: ProcessedFeaturesRequest, db: Session = None):
    """Save processed sensor features to time-series storage."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        repo = services["timeseries"]

        features = repo.save_processed_features(
            tenant_id=request.tenant_id,
            device_id=request.device_id,
            asset_id=request.asset_id,
            features=request.features,
            feature_timestamp=request.feature_timestamp,
            feature_version=request.feature_version,
        )

        return ProcessedFeaturesResponse(
            id=features.id,
            tenant_id=features.tenant_id,
            device_id=features.device_id,
            asset_id=features.asset_id,
            features=features.features,
            feature_version=features.feature_version,
            created_at=features.created_at,
            feature_timestamp=features.feature_timestamp,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error saving features: {str(e)}")


@app.get("/processed-features/latest/{asset_id}", response_model=ProcessedFeaturesResponse)
def get_latest_features(
    asset_id: str,
    tenant_id: str = Query(...),
    db: Session = None,
):
    """Get latest processed features for an asset."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        repo = services["timeseries"]

        features = repo.get_latest_features(tenant_id, asset_id)
        if not features:
            raise HTTPException(status_code=404, detail="No features found")

        return ProcessedFeaturesResponse(
            id=features.id,
            tenant_id=features.tenant_id,
            device_id=features.device_id,
            asset_id=features.asset_id,
            features=features.features,
            feature_version=features.feature_version,
            created_at=features.created_at,
            feature_timestamp=features.feature_timestamp,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error retrieving features: {str(e)}")


# ============================================================================
# Prediction Endpoints
# ============================================================================


@app.post("/predict", response_model=PredictionDetailResponse)
def predict(request: PredictRequest, db: Session = None):
    """
    Run prediction on latest features for an asset.
    Prediction is saved as pending_review status.
    """
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        timeseries_repo = services["timeseries"]
        pred_service = services["prediction"]

        # Get latest features
        features = timeseries_repo.get_latest_features(request.tenant_id, request.asset_id)
        if not features:
            raise HTTPException(status_code=404, detail="No features found for asset")

        # Run model prediction
        prediction_result = PREDICTOR.predict(
            feature_row=features.features,
            device_id=request.device_id,
            asset_id=request.asset_id,
        )

        # Save prediction as pending review
        prediction = pred_service.save_prediction(
            tenant_id=request.tenant_id,
            device_id=request.device_id,
            asset_id=request.asset_id,
            model_name=PREDICTOR.model_name,
            model_version=PREDICTOR.model_version,
            anomaly_score=prediction_result["anomaly_score"],
            predicted_status=prediction_result["predicted_status"],
        )

        return PredictionDetailResponse(
            prediction_id=prediction.prediction_id,
            tenant_id=prediction.tenant_id,
            device_id=prediction.device_id,
            asset_id=prediction.asset_id,
            model_name=prediction.model_name,
            model_version=prediction.model_version,
            anomaly_score=prediction.anomaly_score,
            predicted_status=prediction.predicted_status,
            review_status=prediction.review_status,
            reviewed=prediction.reviewed,
            timestamp=prediction.created_at,
            severity_level=prediction_result["severity_level"],
            is_anomaly=prediction_result["is_anomaly"],
            recommended_action=prediction_result["recommended_action"],
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error running prediction: {str(e)}")


@app.get("/predictions/pending", response_model=list[PredictionResponse])
def get_pending_predictions(
    tenant_id: str = Query(...),
    limit: int = Query(50, ge=1, le=1000),
    db: Session = None,
):
    """Get predictions pending human review."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        pred_service = services["prediction"]

        predictions = pred_service.get_pending_predictions(tenant_id, limit)
        return [
            PredictionResponse(
                prediction_id=p.prediction_id,
                tenant_id=p.tenant_id,
                device_id=p.device_id,
                asset_id=p.asset_id,
                model_name=p.model_name,
                model_version=p.model_version,
                anomaly_score=p.anomaly_score,
                predicted_status=p.predicted_status,
                review_status=p.review_status,
                reviewed=p.reviewed,
                timestamp=p.created_at,
            )
            for p in predictions
        ]
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/predictions/{prediction_id}", response_model=PredictionResponse)
def get_prediction(
    prediction_id: str,
    tenant_id: str = Query(...),
    db: Session = None,
):
    """Get a specific prediction by ID."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        pred_service = services["prediction"]

        prediction = pred_service.get_prediction_by_id(tenant_id, prediction_id)
        if not prediction:
            raise HTTPException(status_code=404, detail="Prediction not found")

        return PredictionResponse(
            prediction_id=prediction.prediction_id,
            tenant_id=prediction.tenant_id,
            device_id=prediction.device_id,
            asset_id=prediction.asset_id,
            model_name=prediction.model_name,
            model_version=prediction.model_version,
            anomaly_score=prediction.anomaly_score,
            predicted_status=prediction.predicted_status,
            review_status=prediction.review_status,
            reviewed=prediction.reviewed,
            timestamp=prediction.created_at,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# ============================================================================
# Review Endpoints
# ============================================================================


@app.post("/predictions/{prediction_id}/review", response_model=PredictionReviewResponse)
def review_prediction(
    prediction_id: str,
    request: ReviewPredictionRequest,
    tenant_id: str = Query(...),
    db: Session = None,
):
    """
    Review a prediction and store human correction.
    Updates prediction status to reviewed.
    """
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        pred_service = services["prediction"]
        review_service = services["review"]

        # Get prediction
        prediction = pred_service.get_prediction_by_id(tenant_id, prediction_id)
        if not prediction:
            raise HTTPException(status_code=404, detail="Prediction not found")

        # Store review
        review = review_service.review_prediction(
            tenant_id=tenant_id,
            prediction_id=prediction_id,
            device_id=request.device_id,
            asset_id=request.asset_id,
            model_prediction=prediction.predicted_status,
            reviewed_label=request.reviewed_label,
            reviewed_by=request.reviewed_by,
            review_comment=request.review_comment,
            is_training_eligible=request.is_training_eligible,
        )

        # Update prediction status
        pred_service.update_prediction_status(
            tenant_id,
            prediction_id,
            "reviewed",
            True,
        )

        return PredictionReviewResponse(
            review_id=review.review_id,
            prediction_id=review.prediction_id,
            tenant_id=review.tenant_id,
            device_id=review.device_id,
            asset_id=review.asset_id,
            model_prediction=review.model_prediction,
            reviewed_label=review.reviewed_label,
            reviewed_by=review.reviewed_by,
            review_comment=review.review_comment,
            is_training_eligible=review.is_training_eligible,
            reviewed_at=review.reviewed_at,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error saving review: {str(e)}")


@app.get("/reviews", response_model=list[PredictionReviewResponse])
def get_reviews(
    tenant_id: str = Query(...),
    limit: int = Query(100, ge=1, le=1000),
    db: Session = None,
):
    """Get all reviews for a tenant."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        review_service = services["review"]

        reviews = review_service.get_reviewed_predictions(tenant_id, limit)
        return [
            PredictionReviewResponse(
                review_id=r.review_id,
                prediction_id=r.prediction_id,
                tenant_id=r.tenant_id,
                device_id=r.device_id,
                asset_id=r.asset_id,
                model_prediction=r.model_prediction,
                reviewed_label=r.reviewed_label,
                reviewed_by=r.reviewed_by,
                review_comment=r.review_comment,
                is_training_eligible=r.is_training_eligible,
                reviewed_at=r.reviewed_at,
            )
            for r in reviews
        ]
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/reviews/training-eligible-count")
def get_training_eligible_count(
    tenant_id: str = Query(...),
    db: Session = None,
):
    """Get count of training-eligible reviews."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        review_service = services["review"]

        count = review_service.count_training_eligible_reviews(tenant_id)
        return {"tenant_id": tenant_id, "training_eligible_count": count}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# ============================================================================
# Retraining Configuration Endpoints
# ============================================================================


@app.get("/retraining/config", response_model=RetrainingConfigResponse)
def get_retraining_config(
    tenant_id: str = Query(...),
    db: Session = None,
):
    """Get retraining configuration for a tenant."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        retraining_service = services["retraining"]

        config = retraining_service.get_config(tenant_id)
        if not config:
            raise HTTPException(status_code=404, detail="Configuration not found")

        return RetrainingConfigResponse(
            tenant_id=config.tenant_id,
            minimum_reviewed_records=config.minimum_reviewed_records,
            auto_retrain_enabled=config.auto_retrain_enabled,
            requires_manual_approval=config.requires_manual_approval,
            updated_by=config.updated_by,
            updated_at=config.updated_at,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.put("/retraining/config", response_model=RetrainingConfigResponse)
def set_retraining_config(
    tenant_id: str = Query(...),
    request: RetrainingConfigRequest = None,
    updated_by: str = Query(...),
    db: Session = None,
):
    """Set or update retraining configuration."""
    if db is None:
        db = get_db()
    if request is None:
        request = RetrainingConfigRequest()
    try:
        services = get_services(db)
        retraining_service = services["retraining"]

        config = retraining_service.set_config(
            tenant_id=tenant_id,
            minimum_reviewed_records=request.minimum_reviewed_records,
            auto_retrain_enabled=request.auto_retrain_enabled,
            requires_manual_approval=request.requires_manual_approval,
            updated_by=updated_by,
        )

        return RetrainingConfigResponse(
            tenant_id=config.tenant_id,
            minimum_reviewed_records=config.minimum_reviewed_records,
            auto_retrain_enabled=config.auto_retrain_enabled,
            requires_manual_approval=config.requires_manual_approval,
            updated_by=config.updated_by,
            updated_at=config.updated_at,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/retraining/eligibility", response_model=RetrainingEligibilityResponse)
def check_retraining_eligibility(
    tenant_id: str = Query(...),
    db: Session = None,
):
    """Check if model retraining is eligible."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        retraining_service = services["retraining"]

        result = retraining_service.check_retraining_eligibility(tenant_id)
        return RetrainingEligibilityResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/retraining/request", response_model=RetrainingRequestResponse)
def create_retraining_request(
    tenant_id: str = Query(...),
    requested_by: str = Query(...),
    db: Session = None,
):
    """Create a retraining request."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        retraining_service = services["retraining"]

        request_obj = retraining_service.create_retraining_request(
            tenant_id=tenant_id,
            requested_by=requested_by,
        )

        return RetrainingRequestResponse(
            request_id=request_obj.request_id,
            tenant_id=request_obj.tenant_id,
            status=request_obj.status,
            training_data_count=request_obj.training_data_count or 0,
            requested_by=request_obj.requested_by,
            approved_by=request_obj.approved_by,
            created_at=request_obj.created_at,
            updated_at=request_obj.updated_at,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/retraining/{request_id}/approve", response_model=RetrainingRequestResponse)
def approve_retraining(
    request_id: str,
    tenant_id: str = Query(...),
    approved_by: str = Query(...),
    db: Session = None,
):
    """Approve a retraining request."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        retraining_service = services["retraining"]

        request_obj = retraining_service.approve_retraining_request(
            tenant_id=tenant_id,
            request_id=request_id,
            approved_by=approved_by,
        )

        if not request_obj:
            raise HTTPException(status_code=404, detail="Request not found")

        return RetrainingRequestResponse(
            request_id=request_obj.request_id,
            tenant_id=request_obj.tenant_id,
            status=request_obj.status,
            training_data_count=request_obj.training_data_count or 0,
            requested_by=request_obj.requested_by,
            approved_by=request_obj.approved_by,
            created_at=request_obj.created_at,
            updated_at=request_obj.updated_at,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# ============================================================================
# Model Version Endpoints
# ============================================================================


@app.get("/models/versions", response_model=list[ModelVersionResponse])
def get_model_versions(
    tenant_id: str = Query(...),
    status: Optional[str] = Query(None, description="Filter by deployment status"),
    limit: int = Query(100, ge=1, le=1000),
    db: Session = None,
):
    """Get model versions for a tenant."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        model_service = services["model_version"]

        if status:
            versions = model_service.get_model_versions_by_status(tenant_id, status, limit)
        else:
            versions = []

        return [
            ModelVersionResponse(
                model_id=v.model_id,
                model_name=v.model_name,
                model_version=v.model_version,
                deployment_status=v.deployment_status,
                training_data_count=v.training_data_count,
                training_date=v.training_date,
                validation_score=v.validation_score,
                approved_by=v.approved_by,
                created_at=v.created_at,
            )
            for v in versions
        ]
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/models/{model_id}/{version}/approve", response_model=ModelVersionResponse)
def approve_model_version(
    model_id: str,
    version: str,
    tenant_id: str = Query(...),
    approved_by: str = Query(...),
    db: Session = None,
):
    """Approve a model version for deployment."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        model_service = services["model_version"]

        mv = model_service.approve_model_version(tenant_id, model_id, version, approved_by)
        if not mv:
            raise HTTPException(status_code=404, detail="Model version not found")

        return ModelVersionResponse(
            model_id=mv.model_id,
            model_name=mv.model_name,
            model_version=mv.model_version,
            deployment_status=mv.deployment_status,
            training_data_count=mv.training_data_count,
            training_date=mv.training_date,
            validation_score=mv.validation_score,
            approved_by=mv.approved_by,
            created_at=mv.created_at,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/models/{model_id}/{version}/deploy", response_model=ModelVersionResponse)
def deploy_model_version(
    model_id: str,
    version: str,
    tenant_id: str = Query(...),
    db: Session = None,
):
    """Deploy a model version (set as active)."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        model_service = services["model_version"]

        mv = model_service.deploy_model_version(tenant_id, model_id, version)
        if not mv:
            raise HTTPException(status_code=404, detail="Model version not found")

        return ModelVersionResponse(
            model_id=mv.model_id,
            model_name=mv.model_name,
            model_version=mv.model_version,
            deployment_status=mv.deployment_status,
            training_data_count=mv.training_data_count,
            training_date=mv.training_date,
            validation_score=mv.validation_score,
            approved_by=mv.approved_by,
            created_at=mv.created_at,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/models/active")
def get_active_model(
    tenant_id: str = Query(...),
    db: Session = None,
):
    """Get the currently active model for a tenant."""
    if db is None:
        db = get_db()
    try:
        services = get_services(db)
        model_service = services["model_version"]

        active = model_service.get_active_model(tenant_id)
        if not active:
            return {"message": "No active model set"}

        return ModelVersionResponse(
            model_id=active.model_id,
            model_name=active.model_name,
            model_version=active.model_version,
            deployment_status=active.deployment_status,
            training_data_count=active.training_data_count,
            training_date=active.training_date,
            validation_score=active.validation_score,
            approved_by=active.approved_by,
            created_at=active.created_at,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000, reload=True)

