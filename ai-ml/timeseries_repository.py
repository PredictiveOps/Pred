"""
TimeSeriesRepository for managing processed feature data.
Provides an abstraction layer for time-series feature storage and retrieval.
Can be easily replaced with InfluxDB or TimescaleDB in the future.
"""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Optional

from sqlalchemy.orm import Session

from db_models import ProcessedFeatures


class TimeSeriesRepository:
    """Repository for storing and retrieving processed sensor feature data."""

    def __init__(self, session: Session):
        self.session = session

    def save_processed_features(
        self,
        tenant_id: str,
        device_id: str,
        asset_id: str,
        features: dict[str, Any],
        feature_timestamp: datetime,
        feature_version: str = "v1",
    ) -> ProcessedFeatures:
        """
        Save processed features to time-series storage.

        Args:
            tenant_id: Tenant identifier
            device_id: Device identifier
            asset_id: Asset identifier
            features: Dictionary of feature values (rms, kurtosis, temperature, etc.)
            feature_timestamp: When the feature was collected
            feature_version: Schema version for features

        Returns:
            Created ProcessedFeatures record
        """
        processed_feature = ProcessedFeatures(
            tenant_id=tenant_id,
            device_id=device_id,
            asset_id=asset_id,
            features=features,
            feature_timestamp=feature_timestamp,
            feature_version=feature_version,
        )
        self.session.add(processed_feature)
        self.session.commit()
        return processed_feature

    def get_latest_features(
        self,
        tenant_id: str,
        asset_id: str,
    ) -> Optional[ProcessedFeatures]:
        """
        Retrieve latest processed features for an asset.

        Args:
            tenant_id: Tenant identifier
            asset_id: Asset identifier

        Returns:
            Latest ProcessedFeatures record or None if not found
        """
        return (
            self.session.query(ProcessedFeatures)
            .filter(
                ProcessedFeatures.tenant_id == tenant_id,
                ProcessedFeatures.asset_id == asset_id,
            )
            .order_by(ProcessedFeatures.created_at.desc())
            .first()
        )

    def get_features_by_asset(
        self,
        tenant_id: str,
        asset_id: str,
        limit: int = 100,
    ) -> list[ProcessedFeatures]:
        """
        Retrieve recent processed features for an asset.

        Args:
            tenant_id: Tenant identifier
            asset_id: Asset identifier
            limit: Maximum number of records to return

        Returns:
            List of ProcessedFeatures records
        """
        return (
            self.session.query(ProcessedFeatures)
            .filter(
                ProcessedFeatures.tenant_id == tenant_id,
                ProcessedFeatures.asset_id == asset_id,
            )
            .order_by(ProcessedFeatures.created_at.desc())
            .limit(limit)
            .all()
        )

    def get_features_by_device(
        self,
        tenant_id: str,
        device_id: str,
        limit: int = 100,
    ) -> list[ProcessedFeatures]:
        """
        Retrieve recent processed features for a device.

        Args:
            tenant_id: Tenant identifier
            device_id: Device identifier
            limit: Maximum number of records to return

        Returns:
            List of ProcessedFeatures records
        """
        return (
            self.session.query(ProcessedFeatures)
            .filter(
                ProcessedFeatures.tenant_id == tenant_id,
                ProcessedFeatures.device_id == device_id,
            )
            .order_by(ProcessedFeatures.created_at.desc())
            .limit(limit)
            .all()
        )
