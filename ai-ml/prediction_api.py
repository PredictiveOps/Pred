from __future__ import annotations

from pathlib import Path
from typing import Any, Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from prediction_module import BearingAnomalyPredictor

PROJECT_ROOT = Path(__file__).resolve().parent
MODEL_DIR = PROJECT_ROOT / "results" / "models"

PREDICTOR = BearingAnomalyPredictor(
    model_path=MODEL_DIR / "vibration_isolation_forest.pkl",
    feature_columns_path=MODEL_DIR / "feature_columns.json",
    thresholds_path=MODEL_DIR / "anomaly_thresholds.json",
)

app = FastAPI(title="Bearing Anomaly Prediction API")


class PredictRequest(BaseModel):
    device_id: str = Field(..., description="Unique identifier for the device")
    asset_id: str = Field(..., description="Unique identifier for the asset")
    features: dict[str, Any] = Field(
        ..., description="Feature values keyed by the model feature names"
    )


class PredictResponse(BaseModel):
    device_id: str
    asset_id: str
    model_name: str
    model_version: str
    anomaly_score: float
    predicted_status: str
    severity_level: int
    is_anomaly: Optional[bool]
    recommended_action: str
    timestamp: str


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok", "service": "bearing_anomaly_prediction"}


@app.post("/predict/vibration", response_model=PredictResponse)
def predict_vibration(request: PredictRequest) -> PredictResponse:
    try:
        prediction = PREDICTOR.predict(
            feature_row=request.features,
            device_id=request.device_id,
            asset_id=request.asset_id,
        )
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except Exception as exc:
        raise HTTPException(status_code=500, detail="Prediction failed") from exc

    return PredictResponse(**prediction)
