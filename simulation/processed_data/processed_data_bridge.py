#!/usr/bin/env python3
"""Kafka consumer bridge: reads processed data from Kafka topic and POSTs to ML service."""

from __future__ import annotations

import json
import logging
import os
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

# Add project root for imports
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "ai-ml"))

try:
    from kafka import KafkaConsumer
    from kafka.errors import KafkaError
except ImportError:
    print("kafka-python not installed. Run: pip install kafka-python")
    raise

# Configuration
KAFKA_BROKERS = os.getenv("KAFKA_BROKERS", "localhost:9092").split(",")
KAFKA_TOPIC = os.getenv("KAFKA_TOPIC", "processed-data")
# In Docker: use ml-service:8000, outside Docker: use localhost:8004 (mapped port)
ML_SERVICE_URL = os.getenv("ML_SERVICE_URL", "http://localhost:8000/processed-features")
CONSUMER_GROUP = os.getenv("CONSUMER_GROUP", "processed-data-bridge")

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger("processed-data-bridge")


def post_to_ml_service(payload: dict[str, Any]) -> tuple[bool, dict]:
    """POST features to ML service /processed-features endpoint."""
    data = json.dumps(payload).encode("utf-8")
    request = Request(
        ML_SERVICE_URL,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urlopen(request, timeout=10.0) as response:
            response_text = response.read().decode("utf-8")
            try:
                response_body = json.loads(response_text)
            except json.JSONDecodeError:
                response_body = {"raw_response": response_text}
            logger.info(f"Successfully sent to ML service: {payload.get('device_id')} -> HTTP {response.status}")
            return True, response_body
    except HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        logger.error(f"HTTP Error {exc.code}: {body}")
        return False, {"error": body}
    except URLError as exc:
        logger.error(f"URL Error: {exc.reason}")
        return False, {"error": str(exc.reason)}
    except Exception as exc:
        logger.error(f"Unexpected error: {exc}")
        return False, {"error": str(exc)}


def transform_to_ml_format(kafka_message: dict) -> dict | None:
    """Transform Kafka message to ML service /processed-features format."""
    # Expected Kafka message format from simulator
    device_id = kafka_message.get("device_id")
    tenant_id = kafka_message.get("tenant_id", "demo_tenant")
    asset_id = kafka_message.get("asset_id", f"{device_id}_bearing")
    features = kafka_message.get("features")
    timestamp = kafka_message.get("timestamp")

    if not device_id or not features:
        logger.warning(f"Missing required fields: device_id={device_id}, features={features}")
        return None

    # Parse timestamp or use current time
    try:
        if timestamp:
            feature_timestamp = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
        else:
            feature_timestamp = datetime.now(timezone.utc)
    except (ValueError, TypeError):
        feature_timestamp = datetime.now(timezone.utc)

    return {
        "tenant_id": tenant_id,
        "device_id": device_id,
        "asset_id": asset_id,
        "features": features,
        "feature_timestamp": feature_timestamp.isoformat(),
        "feature_version": kafka_message.get("feature_version", "v1"),
    }


def main() -> None:
    logger.info(f"Starting bridge: {KAFKA_TOPIC} -> {ML_SERVICE_URL}")
    logger.info(f"Kafka brokers: {KAFKA_BROKERS}")
    logger.info(f"Consumer group: {CONSUMER_GROUP}")

    try:
        consumer = KafkaConsumer(
            KAFKA_TOPIC,
            bootstrap_servers=KAFKA_BROKERS,
            group_id=CONSUMER_GROUP,
            auto_offset_reset="latest",
            value_deserializer=lambda m: json.loads(m.decode("utf-8")),
        )
    except Exception as exc:
        logger.error(f"Failed to connect to Kafka: {exc}")
        sys.exit(1)

    logger.info(f"Connected to Kafka, waiting for messages on topic: {KAFKA_TOPIC}")

    try:
        for message in consumer:
            logger.debug(f"Received message: offset={message.offset}, value={message.value}")

            # Transform to ML service format
            ml_payload = transform_to_ml_format(message.value)
            if not ml_payload:
                logger.warning(f"Skipping invalid message at offset {message.offset}")
                continue

            # Send to ML service
            success, response = post_to_ml_service(ml_payload)
            if not success:
                logger.error(f"Failed to send to ML service: {response}")
                # Don't commit offset on failure - will retry
                continue

            logger.info(f"Processed offset {message.offset}: {ml_payload['device_id']}")

    except KeyboardInterrupt:
        logger.info("Shutting down...")
    finally:
        consumer.close()
        logger.info("Consumer closed")


if __name__ == "__main__":
    main()
