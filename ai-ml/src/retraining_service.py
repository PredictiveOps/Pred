"""
RetrainingService for managing model retraining workflow.
Handles eligibility checking, request creation, approval, and retraining coordination.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Optional

from sqlalchemy.orm import Session

from db_models import (
    RetrainingConfig,
    RetrainingRequest,
    RetrainingStatus,
)
from review_service import ReviewService


class RetrainingService:
    """Service for managing model retraining workflow."""

    def __init__(self, session: Session, review_service: ReviewService):
        self.session = session
        self.review_service = review_service

    def get_config(self, tenant_id: str) -> Optional[RetrainingConfig]:
        """
        Get retraining configuration for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            RetrainingConfig record or None if not found
        """
        return (
            self.session.query(RetrainingConfig)
            .filter(RetrainingConfig.tenant_id == tenant_id)
            .first()
        )

    def set_config(
        self,
        tenant_id: str,
        minimum_reviewed_records: int = 500,
        auto_retrain_enabled: bool = False,
        requires_manual_approval: bool = True,
        updated_by: Optional[str] = None,
    ) -> RetrainingConfig:
        """
        Set or update retraining configuration.

        Args:
            tenant_id: Tenant identifier
            minimum_reviewed_records: Minimum eligible reviews required for retraining
            auto_retrain_enabled: Whether to automatically trigger retraining
            requires_manual_approval: Whether manual approval is required
            updated_by: User ID making the update

        Returns:
            RetrainingConfig record (created or updated)
        """
        config = self.get_config(tenant_id)
        if config:
            config.minimum_reviewed_records = minimum_reviewed_records
            config.auto_retrain_enabled = auto_retrain_enabled
            config.requires_manual_approval = requires_manual_approval
            config.updated_by = updated_by
            config.updated_at = datetime.now(timezone.utc)
        else:
            config = RetrainingConfig(
                tenant_id=tenant_id,
                minimum_reviewed_records=minimum_reviewed_records,
                auto_retrain_enabled=auto_retrain_enabled,
                requires_manual_approval=requires_manual_approval,
                updated_by=updated_by,
            )
            self.session.add(config)
        self.session.commit()
        return config

    def check_retraining_eligibility(
        self,
        tenant_id: str,
    ) -> dict:
        """
        Check if retraining is eligible based on configuration and available data.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Dictionary with eligibility status and details
        """
        config = self.get_config(tenant_id)
        if not config:
            return {
                "eligible": False,
                "reason": "No retraining configuration found",
                "eligible_count": 0,
                "required_count": 0,
            }

        eligible_count = self.review_service.count_training_eligible_reviews(tenant_id)
        label_counts = self.review_service.count_reviews_by_label(
            tenant_id, training_eligible_only=True
        )

        # Check if we have minimum reviews
        if eligible_count < config.minimum_reviewed_records:
            return {
                "eligible": False,
                "reason": f"Insufficient training data. Have {eligible_count}, need {config.minimum_reviewed_records}",
                "eligible_count": eligible_count,
                "required_count": config.minimum_reviewed_records,
                "label_distribution": label_counts,
            }

        # Check if we have at least some samples of each label
        required_labels = {"normal", "warning", "critical"}
        available_labels = set(label_counts.keys())
        missing_labels = required_labels - available_labels

        if missing_labels:
            return {
                "eligible": False,
                "reason": f"Missing label types: {', '.join(missing_labels)}",
                "eligible_count": eligible_count,
                "required_count": config.minimum_reviewed_records,
                "label_distribution": label_counts,
                "missing_labels": list(missing_labels),
            }

        return {
            "eligible": True,
            "eligible_count": eligible_count,
            "required_count": config.minimum_reviewed_records,
            "label_distribution": label_counts,
        }

    def create_retraining_request(
        self,
        tenant_id: str,
        requested_by: str,
        training_data_count: Optional[int] = None,
    ) -> RetrainingRequest:
        """
        Create a new retraining request.

        Args:
            tenant_id: Tenant identifier
            requested_by: User ID requesting retraining
            training_data_count: Number of training records to use

        Returns:
            Created RetrainingRequest record
        """
        if training_data_count is None:
            training_data_count = self.review_service.count_training_eligible_reviews(tenant_id)

        request = RetrainingRequest(
            request_id=str(uuid.uuid4()),
            tenant_id=tenant_id,
            status=RetrainingStatus.CREATED,
            training_data_count=training_data_count,
            requested_by=requested_by,
        )
        self.session.add(request)
        self.session.commit()
        return request

    def approve_retraining_request(
        self,
        tenant_id: str,
        request_id: str,
        approved_by: str,
    ) -> Optional[RetrainingRequest]:
        """
        Approve a retraining request.

        Args:
            tenant_id: Tenant identifier
            request_id: Request identifier
            approved_by: User ID approving the request

        Returns:
            Updated RetrainingRequest record or None if not found
        """
        request = (
            self.session.query(RetrainingRequest)
            .filter(
                RetrainingRequest.tenant_id == tenant_id,
                RetrainingRequest.request_id == request_id,
            )
            .first()
        )
        if request:
            request.status = RetrainingStatus.APPROVED
            request.approved_by = approved_by
            request.updated_at = datetime.now(timezone.utc)
            self.session.commit()
        return request

    def reject_retraining_request(
        self,
        tenant_id: str,
        request_id: str,
        rejection_reason: str,
    ) -> Optional[RetrainingRequest]:
        """
        Reject a retraining request.

        Args:
            tenant_id: Tenant identifier
            request_id: Request identifier
            rejection_reason: Reason for rejection

        Returns:
            Updated RetrainingRequest record or None if not found
        """
        request = (
            self.session.query(RetrainingRequest)
            .filter(
                RetrainingRequest.tenant_id == tenant_id,
                RetrainingRequest.request_id == request_id,
            )
            .first()
        )
        if request:
            request.status = RetrainingStatus.REJECTED
            request.rejection_reason = rejection_reason
            request.updated_at = datetime.now(timezone.utc)
            self.session.commit()
        return request

    def update_request_status(
        self,
        tenant_id: str,
        request_id: str,
        status: str,
    ) -> Optional[RetrainingRequest]:
        """
        Update retraining request status.

        Args:
            tenant_id: Tenant identifier
            request_id: Request identifier
            status: New status

        Returns:
            Updated RetrainingRequest record or None if not found
        """
        request = (
            self.session.query(RetrainingRequest)
            .filter(
                RetrainingRequest.tenant_id == tenant_id,
                RetrainingRequest.request_id == request_id,
            )
            .first()
        )
        if request:
            request.status = status
            if status == RetrainingStatus.COMPLETED:
                request.completed_at = datetime.now(timezone.utc)
            request.updated_at = datetime.now(timezone.utc)
            self.session.commit()
        return request

    def get_request(
        self,
        tenant_id: str,
        request_id: str,
    ) -> Optional[RetrainingRequest]:
        """
        Retrieve a retraining request.

        Args:
            tenant_id: Tenant identifier
            request_id: Request identifier

        Returns:
            RetrainingRequest record or None if not found
        """
        return (
            self.session.query(RetrainingRequest)
            .filter(
                RetrainingRequest.tenant_id == tenant_id,
                RetrainingRequest.request_id == request_id,
            )
            .first()
        )
