"""
Kafka producer for publishing ML predictions.

Publishes prediction results to Kafka for downstream consumers:
- Database persistence service
- Notifications service
- Real-time dashboard updates
"""

from __future__ import annotations

import json
import logging
import os
from datetime import datetime, timezone
from typing import Any, Optional

from kafka import KafkaProducer
from kafka.errors import KafkaError

logger = logging.getLogger("kafka_producer")

# Configuration from environment
KAFKA_BROKERS = os.getenv("KAFKA_BROKERS", "localhost:9092").split(",")
KAFKA_PREDICTIONS_TOPIC = os.getenv("KAFKA_PREDICTIONS_TOPIC", "predictions")
KAFKA_ENABLED = os.getenv("KAFKA_ENABLED", "true").lower() == "true"

# Lazy-initialized producer instance
_producer: Optional[KafkaProducer] = None


def get_producer() -> Optional[KafkaProducer]:
    """Get or create the Kafka producer instance."""
    global _producer

    if not KAFKA_ENABLED:
        logger.debug("Kafka publishing is disabled")
        return None

    if _producer is None:
        try:
            _producer = KafkaProducer(
                bootstrap_servers=KAFKA_BROKERS,
                value_serializer=lambda v: json.dumps(v, default=_json_serializer).encode("utf-8"),
                acks="all",  # Wait for all replicas to acknowledge
                retries=3,
                max_in_flight_requests_per_connection=1,  # Must be 1 for idempotence
                enable_idempotence=True,  # Prevent duplicates on retry
            )
            logger.info(f"Kafka producer connected to {KAFKA_BROKERS}")
        except KafkaError as e:
            logger.error(f"Failed to create Kafka producer: {e}")
            return None

    return _producer


def _json_serializer(obj: Any) -> Any:
    """Custom JSON serializer for non-serializable types."""
    if isinstance(obj, datetime):
        return obj.isoformat()
    raise TypeError(f"Object of type {type(obj)} is not JSON serializable")


def publish_prediction(prediction_data: dict[str, Any]) -> bool:
    """
    Publish a prediction to Kafka.

    Args:
        prediction_data: Dictionary containing prediction details:
            - prediction_id: Unique prediction identifier
            - tenant_id: Tenant identifier
            - device_id: Device identifier
            - asset_id: Asset identifier
            - model_name: Name of the ML model
            - model_version: Version of the model
            - anomaly_score: Anomaly score from the model
            - predicted_status: Predicted status (normal, warning, critical)
            - severity_level: Severity level (1-3)
            - is_anomaly: Whether an anomaly was detected
            - recommended_action: Recommended action string
            - timestamp: Prediction timestamp
            - features: Original features used for prediction (optional)

    Returns:
        True if published successfully, False otherwise
    """
    if not KAFKA_ENABLED:
        logger.debug("Kafka publishing disabled, skipping")
        return True

    producer = get_producer()
    if producer is None:
        logger.warning("Kafka producer not available, cannot publish prediction")
        return False

    # Build the prediction message
    message = {
        "prediction_id": prediction_data.get("prediction_id"),
        "tenant_id": prediction_data.get("tenant_id", "default"),
        "device_id": prediction_data.get("device_id"),
        "asset_id": prediction_data.get("asset_id"),
        "model_name": prediction_data.get("model_name"),
        "model_version": prediction_data.get("model_version"),
        "anomaly_score": prediction_data.get("anomaly_score"),
        "predicted_status": prediction_data.get("predicted_status"),
        "severity_level": prediction_data.get("severity_level"),
        "is_anomaly": prediction_data.get("is_anomaly"),
        "recommended_action": prediction_data.get("recommended_action"),
        "timestamp": prediction_data.get("timestamp", datetime.now(timezone.utc).isoformat()),
        "published_at": datetime.now(timezone.utc).isoformat(),
    }

    # Include features if provided
    if "features" in prediction_data:
        message["features"] = prediction_data["features"]

    # Use device_id as key for partitioning (same device goes to same partition)
    key = prediction_data.get("device_id", "unknown").encode("utf-8")

    try:
        future = producer.send(
            topic=KAFKA_PREDICTIONS_TOPIC,
            key=key,
            value=message,
        )
        # Wait for confirmation (async but we want to know if it failed)
        future.add_callback(
            lambda metadata: logger.debug(
                f"Prediction {prediction_data.get('prediction_id')} published to "
                f"{metadata.topic}:{metadata.partition}:{metadata.offset}"
            )
        )
        future.add_errback(
            lambda exc: logger.error(f"Failed to publish prediction: {exc}")
        )
        return True
    except KafkaError as e:
        logger.error(f"Failed to publish prediction to Kafka: {e}")
        return False


def close_producer() -> None:
    """Close the Kafka producer connection."""
    global _producer
    if _producer is not None:
        try:
            _producer.flush(timeout=10)
            _producer.close()
            logger.info("Kafka producer closed")
        except Exception as e:
            logger.error(f"Error closing Kafka producer: {e}")
        finally:
            _producer = None
