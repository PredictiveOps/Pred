from __future__ import annotations

import json
import logging
import os
from pathlib import Path
from typing import Any

from dotenv import load_dotenv
from kafka import KafkaConsumer

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


def handle_payload(payload: dict[str, Any], db_engine) -> None:
    tenant_id = payload.get("tenant_id") or "unknown-tenant"
    device_id = payload.get("device_id") or "unknown-device"
    asset_id = payload.get("asset_id") or device_id
    features = payload.get("features") or {}

    result = PREDICTOR.predict(
        feature_row=features,
        device_id=device_id,
        asset_id=asset_id,
    )

    logging.info(
        "prediction device=%s asset=%s status=%s anomaly=%s score=%.4f",
        device_id,
        asset_id,
        result.get("predicted_status"),
        result.get("is_anomaly"),
        result.get("anomaly_score", 0.0),
    )

    session = get_session(db_engine)
    try:
        pred_service = PredictionService(session)
        pred_service.save_prediction(
            tenant_id=tenant_id,
            device_id=device_id,
            asset_id=asset_id,
            model_name=PREDICTOR.model_name,
            model_version=PREDICTOR.model_version,
            anomaly_score=result["anomaly_score"],
            predicted_status=result["predicted_status"],
        )
    finally:
        session.close()


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

    consumer = KafkaConsumer(
        topic,
        bootstrap_servers=brokers,
        group_id=group_id,
        enable_auto_commit=True,
        auto_offset_reset=offset_reset,
        value_deserializer=lambda v: json.loads(v.decode("utf-8")),
    )

    logging.info("consuming topic=%s brokers=%s group=%s", topic, brokers, group_id)

    for message in consumer:
        try:
            handle_payload(message.value, db_engine)
        except Exception as exc:
            logging.exception("failed to handle payload: %s", exc)


if __name__ == "__main__":
    main()
