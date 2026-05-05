"""
PredictionService for managing ML model predictions.
Handles prediction generation, storage, and retrieval in the application database.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Optional

from sqlalchemy.orm import Session

from db_models import Prediction, ReviewStatus


class PredictionService:
    """Service for managing ML predictions."""

    def __init__(self, session: Session):
        self.session = session

    def save_prediction(
        self,
        tenant_id: str,
        device_id: str,
        asset_id: str,
        model_name: str,
        model_version: str,
        anomaly_score: float,
        predicted_status: str,
    ) -> Prediction:
        """
        Save a prediction to the application database.

        Args:
            tenant_id: Tenant identifier
            device_id: Device identifier
            asset_id: Asset identifier
            model_name: Name of the model used
            model_version: Version of the model
            anomaly_score: Anomaly score from model
            predicted_status: Predicted status (normal, warning, critical)

        Returns:
            Created Prediction record
        """
        prediction = Prediction(
            prediction_id=str(uuid.uuid4()),
            tenant_id=tenant_id,
            device_id=device_id,
            asset_id=asset_id,
            model_name=model_name,
            model_version=model_version,
            anomaly_score=anomaly_score,
            predicted_status=predicted_status,
            review_status=ReviewStatus.PENDING_REVIEW,
            reviewed=False,
        )
        self.session.add(prediction)
        self.session.commit()
        return prediction

    def get_prediction_by_id(
        self,
        tenant_id: str,
        prediction_id: str,
    ) -> Optional[Prediction]:
        """
        Retrieve a prediction by ID.

        Args:
            tenant_id: Tenant identifier
            prediction_id: Prediction identifier

        Returns:
            Prediction record or None if not found
        """
        return (
            self.session.query(Prediction)
            .filter(
                Prediction.tenant_id == tenant_id,
                Prediction.prediction_id == prediction_id,
            )
            .first()
        )

    def get_pending_predictions(
        self,
        tenant_id: str,
        limit: int = 50,
    ) -> list[Prediction]:
        """
        Retrieve predictions pending human review.

        Args:
            tenant_id: Tenant identifier
            limit: Maximum number of records to return

        Returns:
            List of Prediction records with pending_review status
        """
        return (
            self.session.query(Prediction)
            .filter(
                Prediction.tenant_id == tenant_id,
                Prediction.review_status == ReviewStatus.PENDING_REVIEW,
            )
            .order_by(Prediction.created_at.asc())
            .limit(limit)
            .all()
        )

    def get_reviewed_predictions(
        self,
        tenant_id: str,
        limit: int = 50,
    ) -> list[Prediction]:
        """
        Retrieve reviewed predictions.

        Args:
            tenant_id: Tenant identifier
            limit: Maximum number of records to return

        Returns:
            List of reviewed Prediction records
        """
        return (
            self.session.query(Prediction)
            .filter(
                Prediction.tenant_id == tenant_id,
                Prediction.review_status == ReviewStatus.REVIEWED,
            )
            .order_by(Prediction.updated_at.desc())
            .limit(limit)
            .all()
        )

    def update_prediction_status(
        self,
        tenant_id: str,
        prediction_id: str,
        review_status: str,
        reviewed: bool,
    ) -> Optional[Prediction]:
        """
        Update prediction review status.

        Args:
            tenant_id: Tenant identifier
            prediction_id: Prediction identifier
            review_status: New review status
            reviewed: Whether prediction has been reviewed

        Returns:
            Updated Prediction record or None if not found
        """
        prediction = self.get_prediction_by_id(tenant_id, prediction_id)
        if prediction:
            prediction.review_status = review_status
            prediction.reviewed = reviewed
            prediction.updated_at = datetime.now(timezone.utc)
            self.session.commit()
        return prediction

    def get_predictions_by_asset(
        self,
        tenant_id: str,
        asset_id: str,
        limit: int = 100,
    ) -> list[Prediction]:
        """
        Retrieve predictions for a specific asset.

        Args:
            tenant_id: Tenant identifier
            asset_id: Asset identifier
            limit: Maximum number of records to return

        Returns:
            List of Prediction records
        """
        return (
            self.session.query(Prediction)
            .filter(
                Prediction.tenant_id == tenant_id,
                Prediction.asset_id == asset_id,
            )
            .order_by(Prediction.created_at.desc())
            .limit(limit)
            .all()
        )

    def count_predictions_by_status(
        self,
        tenant_id: str,
        status: str,
    ) -> int:
        """
        Count predictions with a specific review status.

        Args:
            tenant_id: Tenant identifier
            status: Review status to count

        Returns:
            Number of predictions with the given status
        """
        return (
            self.session.query(Prediction)
            .filter(
                Prediction.tenant_id == tenant_id,
                Prediction.review_status == status,
            )
            .count()
        )
