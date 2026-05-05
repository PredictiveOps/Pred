#!/usr/bin/env python3
"""
Simulates raw vibration and temperature data and publishes it to the MQTT broker.
This allows end-to-end testing of the ingestion, pipeline, and prediction components.
"""

import argparse
import json
import math
import random
import time
import ssl
from datetime import datetime, timezone

try:
    import paho.mqtt.client as mqtt
except ImportError:
    print("Please install paho-mqtt: pip install paho-mqtt")
    exit(1)

# Default configuration matching docker-compose.yml
MQTT_BROKER = "localhost"
MQTT_PORT = 8883
MQTT_USER = "pred-device"
MQTT_PASS = "dev-device-password"
CA_CERT = "../mosquitto/certs/ca.crt"

def get_args():
    parser = argparse.ArgumentParser(description="Raw Data Simulator for Predictive Maintenance")
    parser.add_argument("--device", type=str, default="demo_device_001", help="Device ID")
    parser.add_argument("--asset", type=str, default="bearing_motor_001", help="Asset ID")
    parser.add_argument("--interval", type=float, default=0.1, help="Interval between readings (seconds)")
    parser.add_argument("--state", type=str, choices=["normal", "warning", "critical"], default="normal", help="Machine state to simulate")
    parser.add_argument("--duration", type=int, default=60, help="Duration to run simulation in seconds (0 for infinite)")
    return parser.parse_args()

def generate_raw_data(device_id, asset_id, state):
    """
    Generates a realistic chunk of raw sensor data.
    Vibration is typically high frequency (e.g. 10 readings per payload), 
    Temperature is low frequency (1 reading).
    """
    # Base params for normal state
    vib_mean, vib_std = 0.0, 0.5
    temp_bearing, temp_atmos = 45.0, 30.0

    if state == "warning":
        vib_std = 1.2
        temp_bearing = 60.0
    elif state == "critical":
        vib_std = 3.5
        temp_bearing = 85.0

    # Add some random noise
    temp_bearing += random.uniform(-2, 2)
    temp_atmos += random.uniform(-1, 1)

    # Generate an array of high frequency vibration data (e.g., 10 samples)
    vibration_x = [random.gauss(vib_mean, vib_std) for _ in range(10)]
    vibration_y = [random.gauss(vib_mean, vib_std) for _ in range(10)]

    return {
        "device_id": device_id,
        "asset_id": asset_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "vibration_x": vibration_x,
        "vibration_y": vibration_y,
        "temperature_bearing": temp_bearing,
        "temperature_atmospheric": temp_atmos
    }

def main():
    args = get_args()

    client = mqtt.Client(client_id=f"sim_{args.device}")
    client.username_pw_set(MQTT_USER, MQTT_PASS)
    
    # Configure TLS
    try:
        client.tls_set(ca_certs=CA_CERT, tls_version=ssl.PROTOCOL_TLSv1_2)
        client.tls_insecure_set(True) # For self-signed certs in dev
    except FileNotFoundError:
        print(f"Warning: CA cert not found at {CA_CERT}. Make sure you are running from the simulation directory, or update CA_CERT path.")
        print("Proceeding without TLS. If broker requires TLS, this will fail.")

    print(f"Connecting to MQTT broker at {MQTT_BROKER}:{MQTT_PORT}...")
    client.connect(MQTT_BROKER, MQTT_PORT, 60)
    client.loop_start()

    topic = f"devices/{args.device}/data"
    print(f"Publishing to topic: {topic}")
    print(f"Simulating state: {args.state}")

    start_time = time.time()
    packets_sent = 0

    try:
        while True:
            current_time = time.time()
            if args.duration > 0 and (current_time - start_time) > args.duration:
                break
            
            payload = generate_raw_data(args.device, args.asset, args.state)
            client.publish(topic, json.dumps(payload), qos=0)
            packets_sent += 1
            
            if packets_sent % 10 == 0:
                print(f"Sent {packets_sent} packets... (State: {args.state})")
                
            time.sleep(args.interval)
            
    except KeyboardInterrupt:
        print("\nSimulation stopped by user.")
    
    print(f"Finished. Total packets sent: {packets_sent}")
    client.loop_stop()
    client.disconnect()

if __name__ == "__main__":
    main()
