"""
Alert Notifier Module - Filters predictions and routes alerts to dedicated Kafka topic.

Consumes prediction messages from 'predictions' topic, filters records where
predicted_status is NOT 'normal' (i.e., 'warning' or 'critical'), and publishes
them to the 'alerts' topic for downstream notification services.
"""

from __future__ import annotations

import json
import logging
import os
import signal
import sys
from datetime import datetime, timezone
from typing import Any, Optional

from kafka import KafkaConsumer, KafkaProducer
from kafka.errors import KafkaError

logger = logging.getLogger("alert_notifier")

# Configuration from environment
KAFKA_BROKERS = os.getenv("KAFKA_BROKERS", "localhost:9092").split(",")
KAFKA_PREDICTIONS_TOPIC = os.getenv("KAFKA_PREDICTIONS_TOPIC", "predictions")
KAFKA_ALERTS_TOPIC = os.getenv("KAFKA_ALERTS_TOPIC", "alerts")
KAFKA_CONSUMER_GROUP = os.getenv("KAFKA_CONSUMER_GROUP", "alert-notifier")
KAFKA_ENABLED = os.getenv("KAFKA_ENABLED", "true").lower() == "true"

# Valid non-normal statuses that should trigger alerts
ALERT_STATUSES = {"warning", "critical"}

# Lazy-initialized instances
_consumer: Optional[KafkaConsumer] = None
_producer: Optional[KafkaProducer] = None


def get_consumer() -> Optional[KafkaConsumer]:
    """Get or create the Kafka consumer instance."""
    global _consumer

    if not KAFKA_ENABLED:
        logger.debug("Kafka is disabled")
        return None

    if _consumer is None:
        try:
            _consumer = KafkaConsumer(
                KAFKA_PREDICTIONS_TOPIC,
                bootstrap_servers=KAFKA_BROKERS,
                group_id=KAFKA_CONSUMER_GROUP,
                auto_offset_reset="latest",
                enable_auto_commit=True,
                value_deserializer=lambda m: json.loads(m.decode("utf-8")),
                key_deserializer=lambda m: m.decode("utf-8") if m else None,
            )
            logger.info(f"Kafka consumer connected to {KAFKA_BROKERS}, topic: {KAFKA_PREDICTIONS_TOPIC}")
        except KafkaError as e:
            logger.error(f"Failed to create Kafka consumer: {e}")
            return None

    return _consumer


def get_producer() -> Optional[KafkaProducer]:
    """Get or create the Kafka producer instance for alerts."""
    global _producer

    if not KAFKA_ENABLED:
        logger.debug("Kafka is disabled")
        return None

    if _producer is None:
        try:
            _producer = KafkaProducer(
                bootstrap_servers=KAFKA_BROKERS,
                value_serializer=lambda v: json.dumps(v, default=_json_serializer).encode("utf-8"),
                key_serializer=lambda k: k.encode("utf-8") if k else None,
                acks="all",
                retries=3,
                max_in_flight_requests_per_connection=1,  # Must be 1 for idempotence
                enable_idempotence=True,
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


def is_alert_worthy(prediction: dict[str, Any]) -> bool:
    """
    Check if a prediction should trigger an alert.
    
    Returns True if predicted_status is NOT 'normal'.
    """
    predicted_status = prediction.get("predicted_status", "").lower()
    return predicted_status in ALERT_STATUSES


def build_alert_message(prediction: dict[str, Any]) -> dict[str, Any]:
    """
    Build an alert message from a prediction.
    
    Produces a message compatible with notifications-service AlertEvent schema:
    - tenant_id: Tenant identifier
    - type: Notification type ("email" or "push")
    - payload: Alert details as JSON
    - recipients: List of recipients (can be empty for broadcast alerts)
    
    Also includes prediction-specific fields for downstream consumers.
    """
    payload = {
        "alert_id": prediction.get("prediction_id"),
        "device_id": prediction.get("device_id"),
        "asset_id": prediction.get("asset_id"),
        "model_name": prediction.get("model_name"),
        "model_version": prediction.get("model_version"),
        "anomaly_score": prediction.get("anomaly_score"),
        "predicted_status": prediction.get("predicted_status"),
        "severity_level": prediction.get("severity_level"),
        "is_anomaly": prediction.get("is_anomaly"),
        "recommended_action": prediction.get("recommended_action"),
        "prediction_timestamp": prediction.get("timestamp"),
        "alert_timestamp": datetime.now(timezone.utc).isoformat(),
        "alert_type": "prediction_anomaly",
        "failure_probability": prediction.get("anomaly_score", 0.0),  # For notifications-service threshold
    }
    
    return {
        # Fields for notifications-service compatibility
        "tenant_id": prediction.get("tenant_id", "default"),
        "type": "push",  # Default to push notification; can be overridden by recipient preferences
        "payload": payload,
        "recipients": [],  # Empty = broadcast to all subscribed users; can be populated by upstream
        
        # Additional fields for other consumers
        "device_id": prediction.get("device_id"),
        "asset_id": prediction.get("asset_id"),
        "predicted_status": prediction.get("predicted_status"),
        "severity_level": prediction.get("severity_level"),
    }


def publish_alert(alert_data: dict[str, Any]) -> bool:
    """
    Publish an alert to the alerts Kafka topic.
    
    Args:
        alert_data: Alert message to publish
        
    Returns:
        True if published successfully, False otherwise
    """
    if not KAFKA_ENABLED:
        logger.debug("Kafka publishing disabled, skipping alert")
        return True

    producer = get_producer()
    if producer is None:
        logger.warning("Kafka producer not available, cannot publish alert")
        return False

    # Use device_id as key for partitioning
    key = alert_data.get("device_id", "unknown")

    try:
        future = producer.send(
            topic=KAFKA_ALERTS_TOPIC,
            key=key,
            value=alert_data,
        )
        future.add_callback(
            lambda metadata: logger.debug(
                f"Alert {alert_data.get('alert_id')} published to "
                f"{metadata.topic}:{metadata.partition}:{metadata.offset}"
            )
        )
        future.add_errback(
            lambda exc: logger.error(f"Failed to publish alert: {exc}")
        )
        return True
    except KafkaError as e:
        logger.error(f"Failed to publish alert to Kafka: {e}")
        return False


def process_prediction(prediction: dict[str, Any]) -> bool:
    """
    Process a single prediction message.
    
    If prediction is alert-worthy (non-normal status), publishes to alerts topic.
    
    Returns:
        True if alert was published, False if skipped or failed
    """
    prediction_id = prediction.get("prediction_id", "unknown")
    predicted_status = prediction.get("predicted_status", "unknown")
    
    logger.debug(f"Processing prediction {prediction_id}, status: {predicted_status}")
    
    if not is_alert_worthy(prediction):
        logger.debug(f"Prediction {prediction_id} has normal status, skipping")
        return False
    
    logger.info(
        f"Alert-worthy prediction detected: {prediction_id}, "
        f"status: {predicted_status}, device: {prediction.get('device_id')}"
    )
    
    alert_message = build_alert_message(prediction)
    return publish_alert(alert_message)


def run_notifier() -> None:
    """
    Run the alert notifier service.
    
    Continuously consumes predictions and routes alerts for non-normal statuses.
    """
    if not KAFKA_ENABLED:
        logger.warning("Kafka is disabled, notifier will not run")
        return

    consumer = get_consumer()
    if consumer is None:
        logger.error("Failed to initialize consumer, exiting")
        return

    producer = get_producer()
    if producer is None:
        logger.error("Failed to initialize producer, exiting")
        return

    logger.info(f"Starting alert notifier, consuming from '{KAFKA_PREDICTIONS_TOPIC}', publishing to '{KAFKA_ALERTS_TOPIC}'")

    shutdown_requested = False

    def handle_signal(signum, frame):
        nonlocal shutdown_requested
        logger.info(f"Received signal {signum}, initiating graceful shutdown")
        shutdown_requested = True

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    try:
        for message in consumer:
            if shutdown_requested:
                break

            try:
                prediction = message.value
                process_prediction(prediction)
            except Exception as e:
                logger.error(f"Error processing message at offset {message.offset}: {e}")
                continue
    except Exception as e:
        logger.error(f"Consumer error: {e}")
    finally:
        close()


def close() -> None:
    """Close Kafka connections."""
    global _consumer, _producer
    
    if _producer is not None:
        try:
            _producer.flush(timeout=10)
            _producer.close()
            logger.info("Kafka producer closed")
        except Exception as e:
            logger.error(f"Error closing producer: {e}")
        finally:
            _producer = None

    if _consumer is not None:
        try:
            _consumer.close()
            logger.info("Kafka consumer closed")
        except Exception as e:
            logger.error(f"Error closing consumer: {e}")
        finally:
            _consumer = None


if __name__ == "__main__":
    # Configure logging
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
        handlers=[logging.StreamHandler(sys.stdout)],
    )
    
    logger.info("Alert Notifier Service starting...")
    run_notifier()
