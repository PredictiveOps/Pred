"""
Prediction Persistence Service - Stores all predictions from Kafka to database.

Consumes prediction messages from 'predictions' topic and persists ALL records
to the database regardless of predicted_status (normal, warning, critical).
This enables historical tracking, analytics, and model retraining.
"""

from __future__ import annotations

import json
import logging
import os
import signal
import sys
import time
from datetime import datetime, timezone
from typing import Any, Optional

from kafka import KafkaConsumer
from kafka.errors import KafkaError
from sqlalchemy.orm import Session

from db_models import Prediction, ReviewStatus, get_db_engine, get_session

logger = logging.getLogger("prediction_persister")

# Configuration from environment
KAFKA_BROKERS = os.getenv("KAFKA_BROKERS", "localhost:9092").split(",")
KAFKA_PREDICTIONS_TOPIC = os.getenv("KAFKA_PREDICTIONS_TOPIC", "predictions")
KAFKA_CONSUMER_GROUP = os.getenv("KAFKA_CONSUMER_GROUP", "prediction-persister")
KAFKA_ENABLED = os.getenv("KAFKA_ENABLED", "true").lower() == "true"
DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://postgres:postgres@localhost:5432/predictions",
)

# Lazy-initialized instances
_consumer: Optional[KafkaConsumer] = None
_db_session: Optional[Session] = None


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


def get_db() -> Optional[Session]:
    """Get or create the database session."""
    global _db_session

    if _db_session is None:
        try:
            engine = get_db_engine(DATABASE_URL)
            _db_session = get_session(engine)
            logger.info(f"Database connected: {DATABASE_URL.split('@')[-1]}")  # Log without credentials
        except Exception as e:
            logger.error(f"Failed to connect to database: {e}")
            return None

    return _db_session


def prediction_exists(session: Session, prediction_id: str) -> bool:
    """Check if a prediction already exists in the database."""
    return (
        session.query(Prediction)
        .filter(Prediction.prediction_id == prediction_id)
        .first()
        is not None
    )


def persist_prediction(prediction_data: dict[str, Any]) -> bool:
    """
    Persist a prediction to the database.
    
    Stores ALL predictions regardless of predicted_status.
    
    Args:
        prediction_data: Prediction message from Kafka containing:
            - prediction_id: Unique identifier
            - tenant_id: Tenant identifier
            - device_id: Device identifier
            - asset_id: Asset identifier
            - model_name: Name of the ML model
            - model_version: Version of the model
            - anomaly_score: Anomaly score from model
            - predicted_status: Predicted status (normal, warning, critical)
            - severity_level: Severity level (optional)
            - is_anomaly: Whether anomaly detected (optional)
            - timestamp: Prediction timestamp
            
    Returns:
        True if persisted successfully, False otherwise
    """
    session = get_db()
    if session is None:
        logger.warning("Database session not available")
        return False

    prediction_id = prediction_data.get("prediction_id")
    if not prediction_id:
        logger.warning("Prediction missing prediction_id, skipping")
        return False

    # Check for duplicate
    if prediction_exists(session, prediction_id):
        logger.debug(f"Prediction {prediction_id} already exists, skipping")
        return True  # Not an error, just already processed

    try:
        prediction = Prediction(
            prediction_id=prediction_id,
            tenant_id=prediction_data.get("tenant_id", "default"),
            device_id=prediction_data.get("device_id", "unknown"),
            asset_id=prediction_data.get("asset_id", "unknown"),
            model_name=prediction_data.get("model_name", "unknown"),
            model_version=prediction_data.get("model_version", "unknown"),
            anomaly_score=float(prediction_data.get("anomaly_score", 0.0)),
            predicted_status=prediction_data.get("predicted_status", "normal"),
            review_status=ReviewStatus.PENDING_REVIEW,
            reviewed=False,
        )
        
        session.add(prediction)
        session.commit()
        
        logger.info(
            f"Persisted prediction {prediction_id}: "
            f"status={prediction.predicted_status}, "
            f"device={prediction.device_id}, "
            f"tenant={prediction.tenant_id}"
        )
        return True
        
    except Exception as e:
        session.rollback()
        logger.error(f"Failed to persist prediction {prediction_id}: {e}")
        return False


def process_message(message_value: dict[str, Any]) -> bool:
    """
    Process a single prediction message from Kafka.
    
    Persists ALL predictions to database regardless of status.
    
    Returns:
        True if processed successfully, False otherwise
    """
    prediction_id = message_value.get("prediction_id", "unknown")
    predicted_status = message_value.get("predicted_status", "unknown")
    
    logger.debug(f"Processing prediction {prediction_id}, status: {predicted_status}")
    
    return persist_prediction(message_value)


def run_persister() -> None:
    """
    Run the prediction persistence service.

    Continuously consumes predictions and stores them to the database.
    """
    if not KAFKA_ENABLED:
        logger.warning("Kafka is disabled, persister will not run")
        return

    # Retry initialization with backoff
    consumer = None
    db = None
    max_retries = 30
    retry_delay = 2

    for attempt in range(max_retries):
        consumer = get_consumer()
        if consumer is not None:
            break
        logger.warning(f"Failed to initialize consumer, retrying ({attempt + 1}/{max_retries})...")
        time.sleep(retry_delay)

    if consumer is None:
        logger.error("Failed to initialize consumer after max retries, exiting")
        sys.exit(1)

    for attempt in range(max_retries):
        db = get_db()
        if db is not None:
            break
        logger.warning(f"Failed to initialize database, retrying ({attempt + 1}/{max_retries})...")
        time.sleep(retry_delay)

    if db is None:
        logger.error("Failed to initialize database after max retries, exiting")
        sys.exit(1)

    logger.info(
        f"Starting prediction persister, consuming from '{KAFKA_PREDICTIONS_TOPIC}', "
        f"storing ALL statuses to database"
    )

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
                process_message(prediction)
            except Exception as e:
                logger.error(f"Error processing message at offset {message.offset}: {e}")
                continue
    except Exception as e:
        logger.error(f"Consumer error: {e}")
        raise  # Re-raise to trigger restart
    finally:
        close()


def close() -> None:
    """Close Kafka and database connections."""
    global _consumer, _db_session

    if _db_session is not None:
        try:
            _db_session.close()
            logger.info("Database session closed")
        except Exception as e:
            logger.error(f"Error closing database session: {e}")
        finally:
            _db_session = None

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

    logger.info("Prediction Persistence Service starting...")

    # Keep restarting on failure to maintain steady state
    while True:
        try:
            run_persister()
            logger.info("Prediction persister exited gracefully, restarting in 5s...")
        except Exception as e:
            logger.error(f"Prediction persister crashed: {e}, restarting in 5s...")
        time.sleep(5)
