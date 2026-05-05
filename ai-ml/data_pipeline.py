#!/usr/bin/env python3
"""
Data Intelligence Pipeline
Consumes raw data from Kafka, performs noise cancellation, windowing, and aggregation.
Formats the data according to feature_columns.json, calls the ML prediction API, 
and saves the results to the database.
"""

import json
import os
import time
import numpy as np
import pandas as pd
from scipy.signal import medfilt
from scipy.stats import skew, kurtosis
from kafka import KafkaConsumer
import psycopg2
from psycopg2.extras import Json
import requests

KAFKA_BROKER = os.getenv("KAFKA_BROKER", "localhost:9092")
KAFKA_TOPIC = os.getenv("KAFKA_TOPIC", "events")
DB_URL = os.getenv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5433/events")
PREDICTION_API_URL = os.getenv("PREDICTION_API_URL", "http://localhost:8000/predict/vibration")
WINDOW_SIZE = int(os.getenv("WINDOW_SIZE", "10")) # Number of packets per window

# Setup Database
def setup_db():
    conn = psycopg2.connect(DB_URL)
    cur = conn.cursor()
    cur.execute("""
        CREATE TABLE IF NOT EXISTS ai_ml_predictions (
            id SERIAL PRIMARY KEY,
            device_id VARCHAR(255) NOT NULL,
            asset_id VARCHAR(255) NOT NULL,
            timestamp TIMESTAMP NOT NULL,
            anomaly_score FLOAT,
            predicted_status VARCHAR(50),
            severity_level INT,
            recommended_action TEXT,
            features JSONB
        )
    """)
    conn.commit()
    return conn

def calculate_features(data_array):
    """Calculates statistical features for an array of data."""
    if len(data_array) == 0:
        return {
            "mean": 0, "standard_deviation": 0, "minimum": 0, "maximum": 0,
            "peak_to_peak": 0, "rms": 0, "skewness": 0, "kurtosis": 0,
            "crest_factor": 0, "energy": 0, "dominant_frequency_index": 0,
            "spectral_energy": 0, "spectral_centroid_index": 0
        }

    arr = np.array(data_array)
    mean = np.mean(arr)
    std = np.std(arr)
    min_v = np.min(arr)
    max_v = np.max(arr)
    peak_to_peak = max_v - min_v
    rms = np.sqrt(np.mean(arr**2))
    skewness = skew(arr) if len(arr) > 2 else 0
    kurt = kurtosis(arr) if len(arr) > 2 else 0
    crest_factor = max_v / rms if rms != 0 else 0
    energy = np.sum(arr**2)
    
    # Simple frequency domain proxies
    fft_vals = np.abs(np.fft.rfft(arr))
    dominant_frequency_index = int(np.argmax(fft_vals)) if len(fft_vals) > 0 else 0
    spectral_energy = float(np.sum(fft_vals**2))
    spectral_centroid_index = float(np.sum(np.arange(len(fft_vals)) * fft_vals) / np.sum(fft_vals)) if np.sum(fft_vals) != 0 else 0

    return {
        "mean": float(mean),
        "standard_deviation": float(std),
        "minimum": float(min_v),
        "maximum": float(max_v),
        "peak_to_peak": float(peak_to_peak),
        "rms": float(rms),
        "skewness": float(skewness),
        "kurtosis": float(kurt),
        "crest_factor": float(crest_factor),
        "energy": float(energy),
        "dominant_frequency_index": int(dominant_frequency_index),
        "spectral_energy": float(spectral_energy),
        "spectral_centroid_index": float(spectral_centroid_index)
    }

def process_window(window_data, conn):
    """Process a window of packets to extract features and predict."""
    device_id = window_data[0].get("device_id", "unknown")
    asset_id = window_data[0].get("asset_id", "unknown")
    
    vibration_x = []
    vibration_y = []
    temp_bearing = []
    temp_atmos = []
    
    for packet in window_data:
        vibration_x.extend(packet.get("vibration_x", []))
        vibration_y.extend(packet.get("vibration_y", []))
        if "temperature_bearing" in packet:
            temp_bearing.append(packet["temperature_bearing"])
        if "temperature_atmospheric" in packet:
            temp_atmos.append(packet["temperature_atmospheric"])

    # 1. Noise Cancellation using Median Filter
    if len(vibration_x) > 3:
        vibration_x = medfilt(vibration_x, kernel_size=3).tolist()
    if len(vibration_y) > 3:
        vibration_y = medfilt(vibration_y, kernel_size=3).tolist()

    # 2. Resultant Vibration
    vibration_resultant = [np.sqrt(x**2 + y**2) for x, y in zip(vibration_x, vibration_y)]

    # 3. Data Aggregation & Feature Calculation
    features = {}
    
    for prefix, data in [("vibration_x", vibration_x), 
                         ("vibration_y", vibration_y), 
                         ("vibration_resultant", vibration_resultant)]:
        stats = calculate_features(data)
        for stat_name, val in stats.items():
            features[f"{prefix}_{stat_name}"] = val

    # Temperature features
    tb_arr = np.array(temp_bearing) if temp_bearing else np.array([0])
    ta_arr = np.array(temp_atmos) if temp_atmos else np.array([0])
    diff_arr = tb_arr - ta_arr

    features["temperature_bearing_mean"] = float(np.mean(tb_arr))
    features["temperature_bearing_min"] = float(np.min(tb_arr))
    features["temperature_bearing_max"] = float(np.max(tb_arr))
    features["temperature_bearing_std"] = float(np.std(tb_arr))
    features["temperature_bearing_trend"] = float(np.polyfit(np.arange(len(tb_arr)), tb_arr, 1)[0]) if len(tb_arr) > 1 else 0.0

    features["temperature_atmospheric_mean"] = float(np.mean(ta_arr))
    features["temperature_atmospheric_min"] = float(np.min(ta_arr))
    features["temperature_atmospheric_max"] = float(np.max(ta_arr))
    features["temperature_atmospheric_std"] = float(np.std(ta_arr))

    features["temperature_difference_mean"] = float(np.mean(diff_arr))
    features["temperature_difference_max"] = float(np.max(diff_arr))
    features["temperature_difference_trend"] = float(np.polyfit(np.arange(len(diff_arr)), diff_arr, 1)[0]) if len(diff_arr) > 1 else 0.0

    # Call ML API
    payload = {
        "device_id": device_id,
        "asset_id": asset_id,
        "features": features
    }
    
    try:
        response = requests.post(PREDICTION_API_URL, json=payload, timeout=5)
        if response.status_code == 200:
            result = response.json()
            print(f"Prediction success for {device_id}: {result['predicted_status']} (score: {result['anomaly_score']:.2f})")
            
            # Save to Database
            cur = conn.cursor()
            cur.execute("""
                INSERT INTO ai_ml_predictions (device_id, asset_id, timestamp, anomaly_score, predicted_status, severity_level, recommended_action, features)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
            """, (
                result["device_id"], result["asset_id"], result["timestamp"],
                result["anomaly_score"], result["predicted_status"], result["severity_level"],
                result["recommended_action"], Json(features)
            ))
            conn.commit()
            cur.close()
        else:
            print(f"Prediction API error {response.status_code}: {response.text}")
    except requests.exceptions.RequestException as e:
        print(f"Failed to connect to ML prediction API: {e}")


def main():
    print(f"Starting Data Intelligence Pipeline")
    print(f"Connecting to Kafka: {KAFKA_BROKER}, Topic: {KAFKA_TOPIC}")
    
    try:
        conn = setup_db()
        print("Database connection and setup successful.")
    except Exception as e:
        print(f"Failed to connect to database: {e}")
        return

    consumer = KafkaConsumer(
        KAFKA_TOPIC,
        bootstrap_servers=[KAFKA_BROKER],
        auto_offset_reset='latest',
        enable_auto_commit=True,
        group_id='ai-ml-pipeline-group',
        value_deserializer=lambda x: json.loads(x.decode('utf-8'))
    )

    windows = {} # dict of list of packets by device_id

    print("Listening for events...")
    for message in consumer:
        data = message.value
        device_id = data.get("device_id", "unknown")
        
        if device_id not in windows:
            windows[device_id] = []
            
        windows[device_id].append(data)
        
        if len(windows[device_id]) >= WINDOW_SIZE:
            print(f"Processing window for device {device_id} ({WINDOW_SIZE} packets)...")
            process_window(windows[device_id], conn)
            windows[device_id] = [] # Reset window

if __name__ == "__main__":
    main()
