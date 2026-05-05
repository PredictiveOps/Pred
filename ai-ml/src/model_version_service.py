"""
ModelVersionService for managing model versioning and deployment.
Handles model metadata tracking, approval workflow, and active model selection.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Optional

from sqlalchemy.orm import Session

from db_models import (
    ActiveModelVersion,
    DeploymentStatus,
    ModelVersion,
)


class ModelVersionService:
    """Service for managing model versions and deployments."""

    def __init__(self, session: Session):
        self.session = session

    def create_model_version(
        self,
        tenant_id: str,
        model_name: str,
        model_version: str,
        model_path: Optional[str] = None,
        training_data_count: Optional[int] = None,
        training_date: Optional[datetime] = None,
        validation_score: Optional[float] = None,
    ) -> ModelVersion:
        """
        Create a new model version record.

        Args:
            tenant_id: Tenant identifier
            model_name: Name of the model
            model_version: Version string (v1, v2, etc.)
            model_path: Path to saved model artifacts
            training_data_count: Number of training samples used
            training_date: When the model was trained
            validation_score: Validation metric score

        Returns:
            Created ModelVersion record
        """
        model_id = f"{model_name}_{str(uuid.uuid4())[:8]}"

        mv = ModelVersion(
            model_id=model_id,
            tenant_id=tenant_id,
            model_name=model_name,
            model_version=model_version,
            model_path=model_path,
            training_data_count=training_data_count,
            training_date=training_date or datetime.now(timezone.utc),
            validation_score=validation_score,
            deployment_status=DeploymentStatus.TRAINED,
        )
        self.session.add(mv)
        self.session.commit()
        return mv

    def get_model_version(
        self,
        tenant_id: str,
        model_id: str,
        version: str,
    ) -> Optional[ModelVersion]:
        """
        Retrieve a specific model version.

        Args:
            tenant_id: Tenant identifier
            model_id: Model identifier
            version: Version string

        Returns:
            ModelVersion record or None if not found
        """
        return (
            self.session.query(ModelVersion)
            .filter(
                ModelVersion.tenant_id == tenant_id,
                ModelVersion.model_id == model_id,
                ModelVersion.model_version == version,
            )
            .first()
        )

    def get_latest_model_version(
        self,
        tenant_id: str,
        model_id: str,
    ) -> Optional[ModelVersion]:
        """
        Retrieve the latest model version.

        Args:
            tenant_id: Tenant identifier
            model_id: Model identifier

        Returns:
            Latest ModelVersion record or None if not found
        """
        return (
            self.session.query(ModelVersion)
            .filter(
                ModelVersion.tenant_id == tenant_id,
                ModelVersion.model_id == model_id,
            )
            .order_by(ModelVersion.created_at.desc())
            .first()
        )

    def get_active_model(self, tenant_id: str) -> Optional[ModelVersion]:
        """
        Retrieve the currently active model for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Active ModelVersion record or None if not set
        """
        active = (
            self.session.query(ActiveModelVersion)
            .filter(ActiveModelVersion.tenant_id == tenant_id)
            .first()
        )
        if not active:
            return None

        return (
            self.session.query(ModelVersion)
            .filter(
                ModelVersion.tenant_id == tenant_id,
                ModelVersion.model_id == active.active_model_id,
                ModelVersion.model_version == active.active_version,
            )
            .first()
        )

    def approve_model_version(
        self,
        tenant_id: str,
        model_id: str,
        version: str,
        approved_by: str,
    ) -> Optional[ModelVersion]:
        """
        Approve a model version for deployment.

        Args:
            tenant_id: Tenant identifier
            model_id: Model identifier
            version: Version string
            approved_by: User ID approving

        Returns:
            Updated ModelVersion record or None if not found
        """
        mv = self.get_model_version(tenant_id, model_id, version)
        if mv:
            mv.deployment_status = DeploymentStatus.APPROVED
            mv.approved_by = approved_by
            mv.updated_at = datetime.now(timezone.utc)
            self.session.commit()
        return mv

    def deploy_model_version(
        self,
        tenant_id: str,
        model_id: str,
        version: str,
    ) -> Optional[ModelVersion]:
        """
        Deploy a model version (set as active).

        Args:
            tenant_id: Tenant identifier
            model_id: Model identifier
            version: Version string

        Returns:
            Updated ModelVersion record or None if not found
        """
        mv = self.get_model_version(tenant_id, model_id, version)
        if mv:
            # Mark previous deployed version as no longer active
            current_active = self.get_active_model(tenant_id)
            if current_active:
                current_active.deployment_status = DeploymentStatus.APPROVED
                current_active.active_until = datetime.now(timezone.utc)

            # Set new version as active
            mv.deployment_status = DeploymentStatus.DEPLOYED
            mv.updated_at = datetime.now(timezone.utc)

            # Update active model pointer
            active = (
                self.session.query(ActiveModelVersion)
                .filter(ActiveModelVersion.tenant_id == tenant_id)
                .first()
            )
            if active:
                active.active_model_id = model_id
                active.active_version = version
                active.updated_at = datetime.now(timezone.utc)
            else:
                active = ActiveModelVersion(
                    tenant_id=tenant_id,
                    active_model_id=model_id,
                    active_version=version,
                )
                self.session.add(active)

            self.session.commit()
        return mv

    def reject_model_version(
        self,
        tenant_id: str,
        model_id: str,
        version: str,
    ) -> Optional[ModelVersion]:
        """
        Reject a model version (mark as not suitable for deployment).

        Args:
            tenant_id: Tenant identifier
            model_id: Model identifier
            version: Version string

        Returns:
            Updated ModelVersion record or None if not found
        """
        mv = self.get_model_version(tenant_id, model_id, version)
        if mv:
            mv.deployment_status = DeploymentStatus.REJECTED
            mv.updated_at = datetime.now(timezone.utc)
            self.session.commit()
        return mv

    def get_model_versions_by_status(
        self,
        tenant_id: str,
        status: str,
        limit: int = 100,
    ) -> list[ModelVersion]:
        """
        Retrieve model versions with a specific deployment status.

        Args:
            tenant_id: Tenant identifier
            status: Deployment status to filter by
            limit: Maximum number of records to return

        Returns:
            List of ModelVersion records
        """
        return (
            self.session.query(ModelVersion)
            .filter(
                ModelVersion.tenant_id == tenant_id,
                ModelVersion.deployment_status == status,
            )
            .order_by(ModelVersion.created_at.desc())
            .limit(limit)
            .all()
        )

    def get_all_model_versions(
        self,
        tenant_id: str,
        model_id: str,
    ) -> list[ModelVersion]:
        """
        Retrieve all versions of a model.

        Args:
            tenant_id: Tenant identifier
            model_id: Model identifier

        Returns:
            List of ModelVersion records
        """
        return (
            self.session.query(ModelVersion)
            .filter(
                ModelVersion.tenant_id == tenant_id,
                ModelVersion.model_id == model_id,
            )
            .order_by(ModelVersion.created_at.desc())
            .all()
        )
