from __future__ import annotations

import json
from pathlib import Path

import pandas as pd

from prediction_module import BearingAnomalyPredictor


def resolve_sample_data_path(project_root: Path) -> Path:
    candidates = [
        project_root / "data" / "processed" / "bearing_features_labeled.csv",
        project_root / "results" / "processed" / "bearing_features_labeled.csv",
    ]

    for candidate in candidates:
        if candidate.exists():
            return candidate

    raise FileNotFoundError(
        "Could not find sample CSV. Checked: "
        + ", ".join(str(path) for path in candidates)
    )


def main() -> None:
    project_root = Path(__file__).resolve().parent

    predictor = BearingAnomalyPredictor(
        model_path=project_root / "results" / "models" / "vibration_isolation_forest.pkl",
        feature_columns_path=project_root / "results" / "models" / "feature_columns.json",
        thresholds_path=project_root / "results" / "models" / "anomaly_thresholds.json",
        model_name="vibration_isolation_forest",
        model_version="v1",
    )

    sample_csv = resolve_sample_data_path(project_root)
    sample_df = pd.read_csv(sample_csv, nrows=1)

    feature_row = {
        column: sample_df.iloc[0][column]
        for column in predictor.artifacts.feature_columns
    }

    prediction = predictor.predict(
        feature_row=feature_row,
        device_id="demo_device_001",
        asset_id="bearing_motor_001",
    )

    # Basic verification that the new decision layer fields are present
    assert "severity_level" in prediction
    assert "is_anomaly" in prediction
    assert "recommended_action" in prediction

    print(json.dumps(prediction, indent=2))


if __name__ == "__main__":
    main()
