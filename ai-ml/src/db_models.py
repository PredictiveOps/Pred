"""
SQLAlchemy database models for the ML prediction pipeline.
Handles storage of processed features, predictions, reviews, and model versioning.
"""

from __future__ import annotations

from datetime import datetime, timezone
from enum import Enum
from typing import Optional

from sqlalchemy import (
    JSON,
    Boolean,
    DateTime,
    Float,
    Index,
    String,
    create_engine,
)
from sqlalchemy.orm import DeclarativeBase, Session


class Base(DeclarativeBase):
    pass


class ReviewStatus(str, Enum):
    PENDING_REVIEW = "pending_review"
    REVIEWED = "reviewed"
    ARCHIVED = "archived"


class DeploymentStatus(str, Enum):
    TRAINED = "trained"
    PENDING_APPROVAL = "pending_approval"
    APPROVED = "approved"
    DEPLOYED = "deployed"
    REJECTED = "rejected"


class RetrainingStatus(str, Enum):
    CREATED = "created"
    APPROVED = "approved"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    FAILED = "failed"
    REJECTED = "rejected"


from sqlalchemy import Column, Integer

class ProcessedFeatures(Base):
    """
    Stores time-series sensor feature data.
    Raw sensor readings are processed into features (RMS, kurtosis, etc.)
    and stored here for ML model consumption.
    """

    __tablename__ = "processed_features"

    id = Column(Integer, primary_key=True)
    tenant_id = Column(String(255), nullable=False)
    device_id = Column(String(255), nullable=False)
    asset_id = Column(String(255), nullable=False)
    features = Column(JSON, nullable=False)  # {rms, kurtosis, crest_factor, spectral_energy, temperature, ...}
    feature_version = Column(String(50), default="v1")
    created_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc))
    feature_timestamp = Column(DateTime, nullable=False)  # when feature was collected

    __table_args__ = (
        Index("ix_processed_features_tenant_asset", tenant_id, asset_id),
        Index("ix_processed_features_timestamp", created_at),
    )


class Prediction(Base):
    """
    Stores ML model predictions before human review.
    Each prediction has a pending_review status until a human reviews it.
    """

    __tablename__ = "predictions"

    id = Column(Integer, primary_key=True)
    prediction_id = Column(String(255), unique=True, nullable=False)  # auto-generated ID
    tenant_id = Column(String(255), nullable=False)
    device_id = Column(String(255), nullable=False)
    asset_id = Column(String(255), nullable=False)
    model_name = Column(String(255), nullable=False)
    model_version = Column(String(50), nullable=False)
    anomaly_score = Column(Float, nullable=False)
    predicted_status = Column(String(50), nullable=False)  # normal, warning, critical
    review_status = Column(String(50), default=ReviewStatus.PENDING_REVIEW, nullable=False)
    reviewed = Column(Boolean, default=False, nullable=False)
    created_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc))
    updated_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc), onupdate=lambda: datetime.now(timezone.utc))

    __table_args__ = (
        Index("ix_predictions_tenant", tenant_id),
        Index("ix_predictions_asset", asset_id, tenant_id),
    )


class PredictionReview(Base):
    """
    Stores human reviews of predictions.
    Only reviewed and training_eligible records are used for retraining.
    """

    __tablename__ = "prediction_reviews"

    id = Column(Integer, primary_key=True)
    review_id = Column(String(255), unique=True, nullable=False)  # auto-generated ID
    tenant_id = Column(String(255), nullable=False)
    prediction_id = Column(String(255), nullable=False, unique=True)
    device_id = Column(String(255), nullable=False)
    asset_id = Column(String(255), nullable=False)
    model_prediction = Column(String(50), nullable=False)  # original model prediction
    reviewed_label = Column(String(50), nullable=False)  # corrected label by human
    reviewed_by = Column(String(255), nullable=False)  # user ID
    review_comment = Column(String(1000))
    is_training_eligible = Column(Boolean, default=True, nullable=False)
    reviewed_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc))
    created_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc))

    __table_args__ = (
        Index("ix_prediction_reviews_tenant", tenant_id),
    )


class RetrainingConfig(Base):
    """
    Configuration for automated retraining triggers.
    Allows authorized users to define thresholds and approval workflows.
    """

    __tablename__ = "retraining_configs"

    id = Column(Integer, primary_key=True)
    tenant_id = Column(String(255), unique=True, nullable=False)
    minimum_reviewed_records = Column(Integer, default=500, nullable=False)
    auto_retrain_enabled = Column(Boolean, default=False, nullable=False)
    requires_manual_approval = Column(Boolean, default=True, nullable=False)
    updated_by = Column(String(255))
    updated_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc), onupdate=lambda: datetime.now(timezone.utc))


class RetrainingRequest(Base):
    """
    Tracks retraining workflow state.
    Manages retraining from creation through approval, training, and deployment.
    """

    __tablename__ = "retraining_requests"

    id = Column(Integer, primary_key=True)
    request_id = Column(String(255), unique=True, nullable=False)
    tenant_id = Column(String(255), nullable=False)
    status = Column(String(50), default=RetrainingStatus.CREATED, nullable=False)
    training_data_count = Column(Integer)
    requested_by = Column(String(255))
    approved_by = Column(String(255), nullable=True)
    rejection_reason = Column(String(1000), nullable=True)
    completed_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc))
    updated_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc), onupdate=lambda: datetime.now(timezone.utc))

    __table_args__ = (
        Index("ix_retraining_requests_tenant", tenant_id),
    )


class ModelVersion(Base):
    """
    Stores versioned model metadata.
    Tracks model training, validation, approval, and deployment status.
    """

    __tablename__ = "model_versions"

    id = Column(Integer, primary_key=True)
    model_id = Column(String(255), nullable=False)
    tenant_id = Column(String(255), nullable=False)
    model_name = Column(String(255), nullable=False)
    model_version = Column(String(50), nullable=False)  # v1, v2, etc.
    model_path = Column(String(500))  # path to saved model artifacts
    training_data_count = Column(Integer)
    training_date = Column(DateTime)
    validation_score = Column(Float, nullable=True)
    approved_by = Column(String(255), nullable=True)
    deployment_status = Column(String(50), default=DeploymentStatus.TRAINED, nullable=False)
    active_until = Column(DateTime, nullable=True)
    created_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc))
    updated_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc), onupdate=lambda: datetime.now(timezone.utc))

    __table_args__ = (
        Index("ix_model_versions_model", model_id, tenant_id),
    )


class ActiveModelVersion(Base):
    """
    Index table tracking the currently active model version per tenant.
    Allows quick lookup of which model version to use for predictions.
    """

    __tablename__ = "active_model_versions"

    id = Column(Integer, primary_key=True)
    tenant_id = Column(String(255), unique=True, nullable=False)
    active_model_id = Column(String(255), nullable=False)
    active_version = Column(String(50), nullable=False)
    updated_at = Column(DateTime, nullable=False, default=lambda: datetime.now(timezone.utc), onupdate=lambda: datetime.now(timezone.utc))


def get_db_engine(database_url: str):
    """Create SQLAlchemy engine from database URL."""
    return create_engine(database_url, echo=False)


def init_db(database_url: str):
    """Initialize database tables."""
    engine = get_db_engine(database_url)
    Base.metadata.create_all(engine)
    return engine


def get_session(engine) -> Session:
    """Get a new database session."""
    from sqlalchemy.orm import sessionmaker
    SessionLocal = sessionmaker(bind=engine, expire_on_commit=False)
    return SessionLocal()
