#!/usr/bin/env python3
"""Replay processed bearing features to a prediction API in near real-time."""

from __future__ import annotations

import argparse
import json
import math
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

import pandas as pd


DEFAULT_CSV = Path("data/processed/bearing_features_sample.csv")
DEFAULT_ENDPOINT = "http://localhost:8000/predict/vibration"
DEFAULT_LOG_PATH = Path("logs/simulation_predictions.jsonl")
STATUS_COLUMNS = {"status", "health_label", "expected_status", "label"}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Replay processed feature rows as simulated real-time packets."
    )
    parser.add_argument(
        "--csv",
        type=Path,
        default=DEFAULT_CSV,
        help="Path to the processed feature CSV.",
    )
    parser.add_argument(
        "--endpoint",
        default=DEFAULT_ENDPOINT,
        help="Prediction API endpoint (default points to the model route).",
    )
    parser.add_argument(
        "--delay",
        type=float,
        default=1.0,
        help="Delay in seconds between packets.",
    )
    parser.add_argument(
        "--log-file",
        type=Path,
        default=DEFAULT_LOG_PATH,
        help="JSONL file to append prediction responses.",
    )
    parser.add_argument(
        "--loop",
        action="store_true",
        help="Loop indefinitely when CSV replay reaches the end.",
    )
    return parser.parse_args()


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def to_json_safe_value(value: Any) -> Any:
    if pd.isna(value):
        return None
    if isinstance(value, float) and (math.isnan(value) or math.isinf(value)):
        return None
    return value


def split_features_and_status(row: pd.Series) -> tuple[dict[str, Any], str | None]:
    expected_status: str | None = None
    features: dict[str, Any] = {}

    for col in row.index:
        value = to_json_safe_value(row[col])
        if col in STATUS_COLUMNS:
            if value is not None:
                expected_status = str(value)
            continue
        features[col] = value

    return features, expected_status


def build_packet(features: dict[str, Any]) -> dict[str, Any]:
    return {
        "device_id": "demo_device_001",
        "asset_id": "bearing_motor_001",
        "timestamp": utc_now_iso(),
        "features": features,
    }


def post_json(endpoint: str, payload: dict[str, Any], timeout: float = 10.0) -> tuple[int | None, Any]:
    data = json.dumps(payload).encode("utf-8")
    request = Request(
        endpoint,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urlopen(request, timeout=timeout) as response:
            response_text = response.read().decode("utf-8")
            try:
                response_body = json.loads(response_text)
            except json.JSONDecodeError:
                response_body = {"raw_response": response_text}
            return response.status, response_body
    except HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        try:
            parsed_body: Any = json.loads(body)
        except json.JSONDecodeError:
            parsed_body = {"error": body}
        return exc.code, parsed_body
    except URLError as exc:
        # Common case: API server is not running or not reachable.
        return None, {"error": f"API request failed: {exc.reason}"}
    except Exception as exc:  # noqa: BLE001
        return None, {"error": f"Unexpected request error: {exc}"}


def ensure_log_dir(log_file: Path) -> None:
    log_file.parent.mkdir(parents=True, exist_ok=True)


def replay(
    data_frame: pd.DataFrame,
    endpoint: str,
    delay_seconds: float,
    log_file: Path,
    loop_mode: bool,
) -> None:
    ensure_log_dir(log_file)

    with log_file.open("a", encoding="utf-8") as log_handle:
        cycle = 0
        while True:
            cycle += 1
            print(f"\n=== Replay cycle {cycle} ===")
            for idx, row in data_frame.iterrows():
                features, expected_status = split_features_and_status(row)
                packet = build_packet(features)
                status_code, response_body = post_json(endpoint, packet)

                print("\n--- Sent packet ---")
                print(json.dumps(packet, indent=2, ensure_ascii=False))
                if expected_status is not None:
                    print(f"Expected status (from CSV): {expected_status}")
                print("--- Prediction response from model ---")
                print(
                    json.dumps(
                        {
                            "status_code": status_code,
                            "response": response_body,
                        },
                        indent=2,
                        ensure_ascii=False,
                    )
                )

                log_record = {
                    "sent_at": utc_now_iso(),
                    "row_index": int(idx),
                    "request": packet,
                    "expected_status": expected_status,
                    "response_status": status_code,
                    "response": response_body,
                }
                log_handle.write(json.dumps(log_record, ensure_ascii=False) + "\n")
                log_handle.flush()

                if status_code is None:
                    print(
                        "[WARN] Prediction API may be down or unreachable. "
                        "Check that FastAPI is running at "
                        f"{endpoint}."
                    )
                elif status_code == 404:
                    print(
                        "[WARN] Endpoint not found. Use /predict/vibration with this app."
                    )

                time.sleep(delay_seconds)

            if not loop_mode:
                print("\nReplay finished (single pass).")
                break


def main() -> None:
    args = parse_args()

    if args.delay < 0:
        raise ValueError("--delay must be >= 0")

    csv_path = args.csv
    log_path = args.log_file
    if not csv_path.is_absolute():
        # Resolve relative paths from project root when running script from anywhere.
        project_root = Path(__file__).resolve().parents[1]
        csv_path = project_root / csv_path
    else:
        project_root = Path(__file__).resolve().parents[1]

    if not log_path.is_absolute():
        log_path = project_root / log_path

    if not csv_path.exists():
        raise FileNotFoundError(f"CSV file not found: {csv_path}")

    data_frame = pd.read_csv(csv_path)
    if data_frame.empty:
        raise ValueError(f"CSV is empty: {csv_path}")

    print(f"Loaded {len(data_frame)} rows from {csv_path}")
    print(f"Sending predictions to: {args.endpoint}")
    print(f"Logging responses to: {log_path}")
    print(f"Loop mode: {args.loop}")

    replay(
        data_frame=data_frame,
        endpoint=args.endpoint,
        delay_seconds=args.delay,
        log_file=log_path,
        loop_mode=args.loop,
    )


if __name__ == "__main__":
    main()
