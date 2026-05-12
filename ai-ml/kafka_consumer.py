from __future__ import annotations

import json
import logging
import os
from pathlib import Path
from typing import Any

from dotenv import load_dotenv
from kafka import KafkaConsumer, KafkaProducer

from db_models import get_session, init_db
from prediction_module import BearingAnomalyPredictor
from prediction_service import PredictionService

load_dotenv()

PROJECT_ROOT = Path(__file__).resolve().parent
MODEL_DIR = PROJECT_ROOT / "results" / "models"

PREDICTOR = BearingAnomalyPredictor(
    model_path=MODEL_DIR / "vibration_isolation_forest.pkl",
    feature_columns_path=MODEL_DIR / "feature_columns.json",
    thresholds_path=MODEL_DIR / "anomaly_thresholds.json",
)


def get_env(key: str, default: str) -> str:
    value = os.getenv(key)
    return value if value else default


def parse_brokers(raw: str) -> list[str]:
    return [entry.strip() for entry in raw.split(",") if entry.strip()]


def enqueue_notification(
    producer: KafkaProducer,
    topic: str,
    tenant_id: str,
    notification_type: str,
    anomaly_score: float,
    predicted_status: str,
    device_id: str,
    asset_id: str,
    recipients: list[dict[str, str]],
) -> None:
    alert_event = {
        "tenant_id": tenant_id,
        "type": notification_type,
        "payload": {
            "failure_probability": anomaly_score,
            "predicted_status": predicted_status,
            "device_id": device_id,
            "asset_id": asset_id,
        },
        "recipients": recipients,
    }
    logging.info(
        "publishing alert topic=%s tenant=%s device=%s recipients=%d",
        topic,
        tenant_id,
        device_id,
        len(recipients),
    )
    future = producer.send(topic, value=alert_event)
    producer.flush()
    record_metadata = future.get(timeout=10)
    logging.info(
        "alert published topic=%s partition=%d offset=%d device=%s status=%s score=%.4f",
        record_metadata.topic,
        record_metadata.partition,
        record_metadata.offset,
        device_id,
        predicted_status,
        anomaly_score,
    )


def handle_payload(payload: dict[str, Any], db_engine, producer: KafkaProducer, notifications_topic: str, notification_type: str) -> None:
    tenant_id = payload.get("tenant_id") or "unknown-tenant"
    device_id = payload.get("device_id") or "unknown-device"
    asset_id = payload.get("asset_id") or device_id
    features = payload.get("features") or {}
    recipients: list[dict[str, str]] = payload.get("recipients") or []

    logging.info(
        "received event tenant=%s device=%s asset=%s features=%d recipients=%d",
        tenant_id,
        device_id,
        asset_id,
        len(features),
        len(recipients),
    )

    result = PREDICTOR.predict(
        feature_row=features,
        device_id=device_id,
        asset_id=asset_id,
    )

    predicted_status = result.get("predicted_status", "normal")
    anomaly_score = result.get("anomaly_score", 0.0)

    logging.info(
        "prediction device=%s asset=%s status=%s anomaly=%s score=%.4f",
        device_id,
        asset_id,
        predicted_status,
        result.get("is_anomaly"),
        anomaly_score,
    )

    session = get_session(db_engine)
    try:
        pred_service = PredictionService(session)
        prediction = pred_service.save_prediction(
            tenant_id=tenant_id,
            device_id=device_id,
            asset_id=asset_id,
            model_name=PREDICTOR.model_name,
            model_version=PREDICTOR.model_version,
            anomaly_score=anomaly_score,
            predicted_status=predicted_status,
        )
        logging.info("prediction saved id=%s tenant=%s device=%s", prediction.prediction_id, tenant_id, device_id)
    finally:
        session.close()

    if predicted_status not in ("warning", "critical"):
        logging.info("skipping notification status=%s device=%s", predicted_status, device_id)
        return

    if not recipients:
        logging.warning("no recipients in payload, skipping notification device=%s tenant=%s", device_id, tenant_id)
        return

    enqueue_notification(
        producer=producer,
        topic=notifications_topic,
        tenant_id=tenant_id,
        notification_type=notification_type,
        anomaly_score=anomaly_score,
        predicted_status=predicted_status,
        device_id=device_id,
        asset_id=asset_id,
        recipients=recipients,
    )


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="[ml-consumer] %(message)s")

    database_url = get_env(
        "DATABASE_URL",
        "postgresql://user:password@localhost:5433/predictions",
    )
    db_engine = init_db(database_url)

    brokers = parse_brokers(get_env("KAFKA_BROKERS", "localhost:9092"))
    topic = get_env("ML_FEATURES_TOPIC", "ml-features")
    group_id = get_env("ML_KAFKA_GROUP_ID", "ml-service")
    offset_reset = get_env("ML_KAFKA_OFFSET_RESET", "latest")
    notifications_topic = get_env("NOTIFICATIONS_TOPIC", "notifications")
    notification_type = get_env("NOTIFICATION_TYPE", "email")

    producer = KafkaProducer(
        bootstrap_servers=brokers,
        value_serializer=lambda v: json.dumps(v).encode("utf-8"),
    )

    consumer = KafkaConsumer(
        topic,
        bootstrap_servers=brokers,
        group_id=group_id,
        enable_auto_commit=True,
        auto_offset_reset=offset_reset,
        value_deserializer=lambda v: json.loads(v.decode("utf-8")),
    )

    logging.info("consuming topic=%s brokers=%s group=%s", topic, brokers, group_id)
    logging.info("publishing alerts to topic=%s type=%s", notifications_topic, notification_type)

    for message in consumer:
        try:
            handle_payload(message.value, db_engine, producer, notifications_topic, notification_type)
        except Exception as exc:
            logging.exception("failed to handle payload: %s", exc)


if __name__ == "__main__":
    main()
