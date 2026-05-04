"""
ReviewService for managing human reviews of predictions.
Handles storing user corrections, training eligibility marking, and review statistics.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Optional

from sqlalchemy import func
from sqlalchemy.orm import Session

from db_models import PredictionReview, ReviewStatus


class ReviewService:
    """Service for managing human reviews of ML predictions."""

    def __init__(self, session: Session):
        self.session = session

    def review_prediction(
        self,
        tenant_id: str,
        prediction_id: str,
        device_id: str,
        asset_id: str,
        model_prediction: str,
        reviewed_label: str,
        reviewed_by: str,
        review_comment: Optional[str] = None,
        is_training_eligible: bool = True,
    ) -> PredictionReview:
        """
        Store a human review of a prediction.

        Args:
            tenant_id: Tenant identifier
            prediction_id: Prediction identifier being reviewed
            device_id: Device identifier
            asset_id: Asset identifier
            model_prediction: Original model prediction
            reviewed_label: Corrected label by human reviewer
            reviewed_by: User ID of reviewer
            review_comment: Optional comment from reviewer
            is_training_eligible: Whether this review should be used for retraining

        Returns:
            Created PredictionReview record
        """
        review = PredictionReview(
            review_id=str(uuid.uuid4()),
            tenant_id=tenant_id,
            prediction_id=prediction_id,
            device_id=device_id,
            asset_id=asset_id,
            model_prediction=model_prediction,
            reviewed_label=reviewed_label,
            reviewed_by=reviewed_by,
            review_comment=review_comment,
            is_training_eligible=is_training_eligible,
        )
        self.session.add(review)
        self.session.commit()
        return review

    def get_reviewed_predictions(
        self,
        tenant_id: str,
        limit: int = 100,
    ) -> list[PredictionReview]:
        """
        Retrieve all reviews for a tenant.

        Args:
            tenant_id: Tenant identifier
            limit: Maximum number of records to return

        Returns:
            List of PredictionReview records
        """
        return (
            self.session.query(PredictionReview)
            .filter(PredictionReview.tenant_id == tenant_id)
            .order_by(PredictionReview.reviewed_at.desc())
            .limit(limit)
            .all()
        )

    def get_training_eligible_reviews(
        self,
        tenant_id: str,
        limit: int = None,
    ) -> list[PredictionReview]:
        """
        Retrieve reviews marked as eligible for training.

        Args:
            tenant_id: Tenant identifier
            limit: Maximum number of records to return

        Returns:
            List of training-eligible PredictionReview records
        """
        query = (
            self.session.query(PredictionReview)
            .filter(
                PredictionReview.tenant_id == tenant_id,
                PredictionReview.is_training_eligible.is_(True),
            )
            .order_by(PredictionReview.reviewed_at.desc())
        )
        if limit:
            query = query.limit(limit)
        return query.all()

    def count_training_eligible_reviews(
        self,
        tenant_id: str,
    ) -> int:
        """
        Count reviews eligible for training.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Number of training-eligible reviews
        """
        return (
            self.session.query(func.count(PredictionReview.id))
            .filter(
                PredictionReview.tenant_id == tenant_id,
                PredictionReview.is_training_eligible.is_(True),
            )
            .scalar()
        )

    def count_reviews_by_label(
        self,
        tenant_id: str,
        training_eligible_only: bool = False,
    ) -> dict[str, int]:
        """
        Count reviews grouped by reviewed_label.

        Args:
            tenant_id: Tenant identifier
            training_eligible_only: Only count training-eligible reviews

        Returns:
            Dictionary mapping labels to counts
        """
        query = self.session.query(
            PredictionReview.reviewed_label,
            func.count(PredictionReview.id),
        ).filter(PredictionReview.tenant_id == tenant_id)

        if training_eligible_only:
            query = query.filter(PredictionReview.is_training_eligible.is_(True))

        results = query.group_by(PredictionReview.reviewed_label).all()
        return {label: count for label, count in results}

    def mark_training_ineligible(
        self,
        tenant_id: str,
        review_id: str,
        reason: Optional[str] = None,
    ) -> Optional[PredictionReview]:
        """
        Mark a review as ineligible for training.

        Args:
            tenant_id: Tenant identifier
            review_id: Review identifier
            reason: Optional reason for ineligibility

        Returns:
            Updated PredictionReview record or None if not found
        """
        review = (
            self.session.query(PredictionReview)
            .filter(
                PredictionReview.tenant_id == tenant_id,
                PredictionReview.review_id == review_id,
            )
            .first()
        )
        if review:
            review.is_training_eligible = False
            self.session.commit()
        return review

    def get_review_by_prediction_id(
        self,
        tenant_id: str,
        prediction_id: str,
    ) -> Optional[PredictionReview]:
        """
        Retrieve review for a specific prediction.

        Args:
            tenant_id: Tenant identifier
            prediction_id: Prediction identifier

        Returns:
            PredictionReview record or None if not found
        """
        return (
            self.session.query(PredictionReview)
            .filter(
                PredictionReview.tenant_id == tenant_id,
                PredictionReview.prediction_id == prediction_id,
            )
            .first()
        )

    def get_reviews_by_asset(
        self,
        tenant_id: str,
        asset_id: str,
        limit: int = 100,
    ) -> list[PredictionReview]:
        """
        Retrieve reviews for a specific asset.

        Args:
            tenant_id: Tenant identifier
            asset_id: Asset identifier
            limit: Maximum number of records to return

        Returns:
            List of PredictionReview records
        """
        return (
            self.session.query(PredictionReview)
            .filter(
                PredictionReview.tenant_id == tenant_id,
                PredictionReview.asset_id == asset_id,
            )
            .order_by(PredictionReview.reviewed_at.desc())
            .limit(limit)
            .all()
        )
