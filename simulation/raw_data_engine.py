#!/usr/bin/env python3
"""High-throughput raw data engine for MQTT telemetry load testing.

This script can publish 900-1000+ messages/second locally when run with
QoS 0 and multiple workers. It supports both unsigned load-test payloads and
signed ingestion-compatible payloads.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import os
import random
import ssl
import threading
import time
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import paho.mqtt.client as mqtt
except ImportError as exc:  # pragma: no cover - import-time guidance
    raise SystemExit("Install paho-mqtt first: pip install paho-mqtt") from exc

try:
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import ec
except ImportError:  # pragma: no cover - optional signed mode only
    hashes = serialization = ec = None


DEFAULT_BROKER = "localhost"
DEFAULT_PORT = 8883
DEFAULT_USERNAME = "pred-device"
DEFAULT_PASSWORD = "dev-device-password"
DEFAULT_CA_CERT = "../mosquitto/certs/ca.crt"
DEFAULT_TOPIC_TEMPLATE = "devices/{device_id}/data"
DEFAULT_TARGET_RATE = 1000.0


@dataclass(frozen=True)
class EngineConfig:
    device_id: str
    asset_id: str
    state: str
    target_rate: float
    duration_seconds: float
    count: int
    workers: int
    topic: str
    broker: str
    port: int
    username: str
    password: str
    ca_cert: str | None
    unsigned: bool
    private_key_path: Path | None
    seed: int | None
    progress_interval: int
    tls_insecure: bool


class AtomicCounter:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._value = 0

    def increment(self) -> int:
        with self._lock:
            self._value += 1
            return self._value

    def value(self) -> int:
        with self._lock:
            return self._value


class StopState:
    def __init__(self) -> None:
        self._event = threading.Event()

    def set(self) -> None:
        self._event.set()

    def is_set(self) -> bool:
        return self._event.is_set()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="High-throughput raw data engine for MQTT telemetry load testing."
    )
    parser.add_argument("--device", default="demo_device_001", help="Device ID used in the MQTT topic")
    parser.add_argument("--asset", default="bearing_motor_001", help="Asset ID included in the payload")
    parser.add_argument(
        "--state",
        choices=["normal", "warning", "critical"],
        default="normal",
        help="Machine state to simulate",
    )
    parser.add_argument(
        "--rate",
        type=float,
        default=DEFAULT_TARGET_RATE,
        help="Target total messages per second across all workers",
    )
    parser.add_argument(
        "--duration",
        type=float,
        default=60.0,
        help="Run duration in seconds; use 0 for unlimited until --count is reached",
    )
    parser.add_argument(
        "--count",
        type=int,
        default=0,
        help="Total message count to send; 0 means no count limit",
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=1,
        help="Number of concurrent MQTT publisher workers",
    )
    parser.add_argument(
        "--topic",
        default="",
        help="MQTT topic template; defaults to devices/{device_id}/data",
    )
    parser.add_argument("--broker", default=DEFAULT_BROKER, help="MQTT broker host")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT, help="MQTT broker port")
    parser.add_argument("--username", default=DEFAULT_USERNAME, help="MQTT username")
    parser.add_argument("--password", default=DEFAULT_PASSWORD, help="MQTT password")
    parser.add_argument(
        "--ca-cert",
        default=DEFAULT_CA_CERT,
        help="CA certificate for TLS broker validation; use empty string to disable",
    )
    parser.add_argument(
        "--private-key",
        type=Path,
        default=None,
        help="Optional PEM private key for signed telemetry payloads",
    )
    parser.add_argument(
        "--unsigned",
        action="store_true",
        help="Send unsigned payloads for broker-only load testing",
    )
    parser.add_argument("--seed", type=int, default=None, help="Base RNG seed for reproducible data")
    parser.add_argument(
        "--progress-interval",
        type=int,
        default=1000,
        help="Print progress every N messages; set to 0 to disable progress output",
    )
    parser.add_argument(
        "--tls-insecure",
        action="store_true",
        help="Disable TLS cert validation for dev/self-signed brokers",
    )
    return parser.parse_args()


def build_config(args: argparse.Namespace) -> EngineConfig:
    if args.rate <= 0:
        raise ValueError("--rate must be greater than 0")
    if args.workers <= 0:
        raise ValueError("--workers must be greater than 0")
    if args.count < 0:
        raise ValueError("--count must be >= 0")
    if args.duration < 0:
        raise ValueError("--duration must be >= 0")

    project_root = Path(__file__).resolve().parents[1]
    topic = args.topic or DEFAULT_TOPIC_TEMPLATE.format(device_id=args.device)
    ca_cert = args.ca_cert.strip() if args.ca_cert else None
    if ca_cert == "":
        ca_cert = None
    elif ca_cert is not None:
        ca_path = Path(ca_cert)
        if not ca_path.is_absolute():
            ca_path = (project_root / ca_path).resolve()
        ca_cert = str(ca_path)

    private_key = args.private_key
    if private_key is not None and not private_key.is_absolute():
        private_key = (project_root / private_key).resolve()

    return EngineConfig(
        device_id=args.device,
        asset_id=args.asset,
        state=args.state,
        target_rate=args.rate,
        duration_seconds=args.duration,
        count=args.count,
        workers=args.workers,
        topic=topic,
        broker=args.broker,
        port=args.port,
        username=args.username,
        password=args.password,
        ca_cert=ca_cert,
        unsigned=args.unsigned,
        private_key_path=private_key,
        seed=args.seed,
        progress_interval=args.progress_interval,
        tls_insecure=args.tls_insecure,
    )


def load_private_key(private_key_path: Path):
    if serialization is None:
        raise RuntimeError(
            "Signed payload mode requires the cryptography package. "
            "Install it with: pip install cryptography"
        )
    key_text = private_key_path.read_text(encoding="utf-8")
    return serialization.load_pem_private_key(key_text.encode("utf-8"), password=None)


def generate_payload_data(state: str, rng: random.Random, sequence: int) -> dict[str, Any]:
    if state == "normal":
        base_v_rms = 1.15
        temp_c = 72.0
        peaks = (50, 100, 150)
        status = "ok"
    elif state == "warning":
        base_v_rms = 2.4
        temp_c = 84.0
        peaks = (65, 120, 180)
        status = "warn"
    else:
        base_v_rms = 4.8
        temp_c = 96.0
        peaks = (80, 140, 210)
        status = "critical"

    data = {
        "mode": state,
        "peak_hz_1": peaks[0] + rng.randint(-2, 2),
        "peak_hz_2": peaks[1] + rng.randint(-2, 2),
        "peak_hz_3": peaks[2] + rng.randint(-2, 2),
        "status": status,
        "temp_c": round(temp_c + rng.uniform(-1.2, 1.2), 3),
        "v_rms": round(base_v_rms + rng.uniform(-0.08, 0.08), 4),
    }
    return data


def canonical_json_bytes(data: dict[str, Any]) -> bytes:
    # Match the canonical ordering used by the ingestion-service signing helper.
    ordered = {
        "mode": data["mode"],
        "peak_hz_1": int(data["peak_hz_1"]),
        "peak_hz_2": int(data["peak_hz_2"]),
        "peak_hz_3": int(data["peak_hz_3"]),
        "status": data["status"],
        "temp_c": float(data["temp_c"]),
        "v_rms": float(data["v_rms"]),
    }
    return json.dumps(ordered, separators=(",", ":"), sort_keys=False).encode("utf-8")


def sign_envelope(private_key, data: dict[str, Any], sequence: int) -> str:
    data_bytes = canonical_json_bytes(data)
    signature = private_key.sign(data_bytes, ec.ECDSA(hashes.SHA256()))
    envelope = {
        "timestamp": int(time.time()),
        "nonce": f"n-{sequence}-{uuid.uuid4().hex}",
        "data": data,
        "signature": base64.b64encode(signature).decode("utf-8"),
    }
    return json.dumps(envelope, separators=(",", ":"), sort_keys=False)


def build_payload(config: EngineConfig, data: dict[str, Any], sequence: int, private_key=None) -> str:
    if config.unsigned:
        envelope = {
            "timestamp": int(time.time()),
            "nonce": f"n-{sequence}-{uuid.uuid4().hex}",
            "data": data,
        }
        return json.dumps(envelope, separators=(",", ":"), sort_keys=False)

    if private_key is None:
        raise RuntimeError("Signed mode requires --private-key or use --unsigned")
    return sign_envelope(private_key, data, sequence)


def create_client(config: EngineConfig, worker_id: int) -> mqtt.Client:
    client = mqtt.Client(client_id=f"raw_engine_{config.device_id}_{worker_id}")
    client.username_pw_set(config.username, config.password)
    if config.ca_cert:
        client.tls_set(ca_certs=config.ca_cert, tls_version=ssl.PROTOCOL_TLSv1_2)
        client.tls_insecure_set(config.tls_insecure)
    return client


def worker_loop(
    worker_id: int,
    config: EngineConfig,
    private_key,
    sent_counter: AtomicCounter,
    stop_state: StopState,
    started_at: float,
) -> int:
    worker_rng = random.Random((config.seed or int(time.time())) + worker_id * 100_000)
    client = create_client(config, worker_id)
    client.connect(config.broker, config.port, 60)
    client.loop_start()

    worker_rate = config.target_rate / config.workers
    interval = 1.0 / worker_rate if worker_rate > 0 else 0.0
    next_tick = time.perf_counter()
    worker_sent = 0

    try:
        while not stop_state.is_set():
            if config.duration_seconds > 0 and (time.perf_counter() - started_at) >= config.duration_seconds:
                stop_state.set()
                break

            if config.count > 0:
                sequence = sent_counter.value() + 1
                if sequence > config.count:
                    stop_state.set()
                    break
            else:
                sequence = sent_counter.value() + 1

            current_sequence = sent_counter.increment()
            if config.count > 0 and current_sequence > config.count:
                stop_state.set()
                break

            data = generate_payload_data(config.state, worker_rng, current_sequence)
            payload = build_payload(config, data, current_sequence, private_key=private_key)
            client.publish(config.topic, payload, qos=0)
            worker_sent += 1

            if config.progress_interval > 0 and current_sequence % config.progress_interval == 0:
                elapsed = max(time.perf_counter() - started_at, 1e-6)
                rate = current_sequence / elapsed
                print(f"[worker {worker_id}] sent={current_sequence} avg_rate={rate:.1f}/s")

            if interval > 0:
                next_tick += interval
                sleep_for = next_tick - time.perf_counter()
                if sleep_for > 0:
                    time.sleep(sleep_for)
                else:
                    next_tick = time.perf_counter()
    finally:
        client.loop_stop()
        client.disconnect()

    return worker_sent


def main() -> None:
    args = parse_args()
    config = build_config(args)

    print("==========================================")
    print("Raw Data Engine")
    print("==========================================")
    print(f"Broker:           {config.broker}:{config.port}")
    print(f"Topic:             {config.topic}")
    print(f"Device/Asset:      {config.device_id} / {config.asset_id}")
    print(f"State:             {config.state}")
    print(f"Target rate:       {config.target_rate:.1f} msg/s total")
    print(f"Workers:           {config.workers}")
    print(f"Duration:          {config.duration_seconds}s")
    print(f"Count limit:       {config.count or 'unlimited'}")
    print(f"Payload mode:      {'unsigned' if config.unsigned else 'signed'}")
    if config.private_key_path:
        print(f"Private key:       {config.private_key_path}")
    print("==========================================")

    private_key = None
    if not config.unsigned:
        key_path = config.private_key_path
        if key_path is None:
            key_env = os.getenv("DEVICE_PRIVATE_KEY")
            if key_env:
                key_path = Path(key_env)
        if key_path is None:
            raise SystemExit(
                "Signed mode needs --private-key or DEVICE_PRIVATE_KEY. "
                "Use --unsigned for broker-only load testing."
            )
        private_key = load_private_key(key_path)

    sent_counter = AtomicCounter()
    stop_state = StopState()
    started_at = time.perf_counter()

    threads: list[threading.Thread] = []
    worker_results: list[int] = [0] * config.workers

    def run_worker(index: int) -> None:
        worker_results[index] = worker_loop(
            worker_id=index + 1,
            config=config,
            private_key=private_key,
            sent_counter=sent_counter,
            stop_state=stop_state,
            started_at=started_at,
        )

    for index in range(config.workers):
        thread = threading.Thread(target=run_worker, args=(index,), daemon=True)
        thread.start()
        threads.append(thread)

    try:
        for thread in threads:
            thread.join()
    except KeyboardInterrupt:
        stop_state.set()
        print("\nStopping engine...")
        for thread in threads:
            thread.join()

    elapsed = max(time.perf_counter() - started_at, 1e-6)
    total_sent = sent_counter.value()
    avg_rate = total_sent / elapsed
    print("==========================================")
    print(f"Finished. Total messages sent: {total_sent}")
    print(f"Average throughput: {avg_rate:.1f} msg/s")
    print("==========================================")


if __name__ == "__main__":
    main()
