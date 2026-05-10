#!/usr/bin/env python3
"""Simulates 3 edge devices sending signed MQTT telemetry to the ingestion service.

Each device:
  1. HTTP-registers itself with the ingestion service (idempotent).
  2. Connects to the MQTT broker and publishes its ECDSA P-256 public key.
  3. Waits for the registration-response confirming the key was accepted.
  4. Continuously publishes signed sensor-telemetry at random intervals.

Environment variables (all optional – defaults match docker-compose dev config):
  INGESTION_HTTP_URL   http://ingestion-service:8003
  MQTT_BROKER_HOST     mosquitto
  MQTT_BROKER_PORT     1883
  MQTT_DEVICE_USERNAME pred-device
  MQTT_DEVICE_PASSWORD dev-device-password
  TENANT_ID            1
  SEND_INTERVAL_MIN    5   (seconds between publishes, lower bound)
  SEND_INTERVAL_MAX    30  (seconds between publishes, upper bound)
"""

from __future__ import annotations

import base64
import json
import logging
import os
import random
import ssl
import threading
import time
import uuid

import paho.mqtt.client as mqtt
import requests
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s [device-%(name)s] %(message)s",
    datefmt="%H:%M:%S",
)

INGESTION_HTTP_URL = os.getenv("INGESTION_HTTP_URL", "http://ingestion-service:8003")
MQTT_BROKER_HOST = os.getenv("MQTT_BROKER_HOST", "mosquitto")
MQTT_BROKER_PORT = int(os.getenv("MQTT_BROKER_PORT", "1883"))
MQTT_USERNAME = os.getenv("MQTT_DEVICE_USERNAME", "pred-device")
MQTT_PASSWORD = os.getenv("MQTT_DEVICE_PASSWORD", "dev-device-password")
TENANT_ID = int(os.getenv("TENANT_ID", "1"))
SEND_INTERVAL_MIN = float(os.getenv("SEND_INTERVAL_MIN", "5"))
SEND_INTERVAL_MAX = float(os.getenv("SEND_INTERVAL_MAX", "30"))
MQTT_CA_CERT = os.getenv("MQTT_CA_CERT", "")

# Three device profiles with distinct sensor characteristics.
# Device 2 runs hot with vibration to exercise warning/error paths.
DEVICE_PROFILES = [
    {
        "device_id": 1,
        "temp_baseline": 65.0,
        "temp_stddev": 2.0,
        "peak_hz": (45, 90, 135),
        "v_rms_baseline": 1.1,
        "mode": "normal",
        "status_weights": [0.85, 0.12, 0.03],  # ok, warning, error
    },
    {
        "device_id": 2,
        "temp_baseline": 78.0,
        "temp_stddev": 4.0,
        "peak_hz": (52, 104, 156),
        "v_rms_baseline": 1.8,
        "mode": "normal",
        "status_weights": [0.60, 0.30, 0.10],
    },
    {
        "device_id": 3,
        "temp_baseline": 55.0,
        "temp_stddev": 1.5,
        "peak_hz": (60, 120, 180),
        "v_rms_baseline": 0.9,
        "mode": "diagnostic",
        "status_weights": [0.90, 0.08, 0.02],
    },
]

_STATUSES = ["ok", "warning", "error"]


class DeviceSimulator:
    def __init__(self, profile: dict) -> None:
        self.device_id: int = profile["device_id"]
        self.profile = profile
        self.log = logging.getLogger(str(self.device_id))

        self._private_key = ec.generate_private_key(ec.SECP256R1())
        self._public_key_pem: str = (
            self._private_key.public_key()
            .public_bytes(
                serialization.Encoding.PEM,
                serialization.PublicFormat.SubjectPublicKeyInfo,
            )
            .decode()
        )

        self._registered = threading.Event()
        self._client: mqtt.Client | None = None

    # ------------------------------------------------------------------ #
    # Sensor data & signing
    # ------------------------------------------------------------------ #

    def _sensor_data_bytes(self) -> bytes:
        """Return JSON bytes for the data object in canonical alphabetical order."""
        p1, p2, p3 = self.profile["peak_hz"]
        jitter = lambda v: int(v * random.uniform(0.9, 1.1))  # noqa: E731
        temp = self.profile["temp_baseline"] + random.gauss(0, self.profile["temp_stddev"])
        v_rms = self.profile["v_rms_baseline"] * random.uniform(0.85, 1.15)
        status = random.choices(_STATUSES, weights=self.profile["status_weights"])[0]

        # Field order MUST match canonical alphabetical order expected by the server.
        data = {
            "mode": self.profile["mode"],
            "peak_hz_1": jitter(p1),
            "peak_hz_2": jitter(p2),
            "peak_hz_3": jitter(p3),
            "status": status,
            "temp_c": round(temp, 2),
            "v_rms": round(v_rms, 4),
        }
        return json.dumps(data, separators=(",", ":")).encode()

    def _sign(self, data: bytes) -> str:
        """ECDSA P-256 / SHA-256 signature, base64-encoded DER (ASN.1)."""
        sig = self._private_key.sign(data, ec.ECDSA(hashes.SHA256()))
        return base64.b64encode(sig).decode()

    def _build_payload(self) -> bytes:
        """Build the full signed MQTT payload.

        The ``data`` bytes are embedded verbatim so the signature covers exactly
        the same bytes the server will extract from ``message.Data``.
        """
        data_bytes = self._sensor_data_bytes()
        signature = self._sign(data_bytes)
        nonce = str(uuid.uuid4())
        timestamp = int(time.time())

        # Manual JSON construction keeps data_bytes byte-perfect.
        return (
            b'{"timestamp":'
            + str(timestamp).encode()
            + b',"nonce":"'
            + nonce.encode()
            + b'","data":'
            + data_bytes
            + b',"signature":"'
            + signature.encode()
            + b'"}'
        )

    # ------------------------------------------------------------------ #
    # MQTT callbacks
    # ------------------------------------------------------------------ #

    def _on_connect(self, client: mqtt.Client, _ud, _flags, rc: int) -> None:
        if rc != 0:
            self.log.error("MQTT connect failed rc=%d", rc)
            return
        self.log.info("MQTT connected to %s:%d", MQTT_BROKER_HOST, MQTT_BROKER_PORT)
        response_topic = f"devices/{self.device_id}/registration/response"
        client.subscribe(response_topic)
        reg_payload = json.dumps({"public_key": self._public_key_pem})
        client.publish(f"devices/{self.device_id}/registration", reg_payload, qos=1)
        self.log.info("Published public-key registration request")

    def _on_message(self, _client, _ud, msg: mqtt.MQTTMessage) -> None:
        expected = f"devices/{self.device_id}/registration/response"
        if msg.topic != expected:
            return
        try:
            body = json.loads(msg.payload)
        except json.JSONDecodeError:
            self.log.warning("Non-JSON on %s", msg.topic)
            return
        if body.get("registration_status") == "ok":
            self.log.info("Key registration confirmed by server")
            self._registered.set()
        else:
            self.log.error("Key registration rejected: %s", body)

    # ------------------------------------------------------------------ #
    # Lifecycle
    # ------------------------------------------------------------------ #

    def _wait_for_ingestion(self, retries: int = 30, delay: float = 2.0) -> None:
        url = f"{INGESTION_HTTP_URL}/health"
        for attempt in range(1, retries + 1):
            try:
                r = requests.get(url, timeout=3)
                if r.status_code < 500:
                    return
            except requests.RequestException:
                pass
            self.log.info("Waiting for ingestion service (%d/%d)…", attempt, retries)
            time.sleep(delay)
        raise RuntimeError("Ingestion service did not become ready")

    def _http_register(self) -> None:
        url = f"{INGESTION_HTTP_URL}/devices/register"
        try:
            r = requests.post(
                url,
                json={"device_id": self.device_id, "tenant_id": TENANT_ID},
                timeout=10,
            )
            if r.status_code == 201:
                self.log.info("Device registered via HTTP")
            else:
                # 5xx usually means the device record already exists; that is fine.
                self.log.warning("HTTP register returned %d (may already exist)", r.status_code)
        except requests.RequestException as exc:
            raise RuntimeError(f"HTTP registration failed: {exc}") from exc

    def run(self) -> None:
        self._wait_for_ingestion()
        self._http_register()

        self._client = mqtt.Client(client_id=str(self.device_id), clean_session=True)
        self._client.username_pw_set(MQTT_USERNAME, MQTT_PASSWORD)
        if MQTT_CA_CERT:
            self._client.tls_set(ca_certs=MQTT_CA_CERT, tls_version=ssl.PROTOCOL_TLS_CLIENT)
            # Cert is issued for localhost; skip hostname check in simulation.
            self._client.tls_insecure_set(True)
        self._client.on_connect = self._on_connect
        self._client.on_message = self._on_message
        self._client.connect(MQTT_BROKER_HOST, MQTT_BROKER_PORT, keepalive=60)
        self._client.loop_start()

        if not self._registered.wait(timeout=30):
            self.log.error("Key registration timed out – check broker ACL and ingestion logs")
            return

        data_topic = f"devices/{self.device_id}/data"
        self.log.info("Telemetry loop started → topic %s", data_topic)
        while True:
            payload = self._build_payload()
            self._client.publish(data_topic, payload, qos=0)
            self.log.info("Published %d bytes", len(payload))
            time.sleep(random.uniform(SEND_INTERVAL_MIN, SEND_INTERVAL_MAX))


def main() -> None:
    threads = [
        threading.Thread(
            target=DeviceSimulator(p).run,
            name=f"device-{p['device_id']}",
            daemon=True,
        )
        for p in DEVICE_PROFILES
    ]
    for t in threads:
        t.start()
        time.sleep(1)  # stagger startup to avoid simultaneous HTTP registration

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
