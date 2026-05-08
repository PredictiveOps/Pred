#!/usr/bin/env python3
"""Raw telemetry engine with switchable payload formats over MQTT.

Publishes JSON messages with the default "new" structure:
  {
    "device_name": "demo_device_001",
    "timestamp": "2026-05-06T12:00:00Z",
    "vibration_x": 0.123,
    "vibration_y": 0.456,
    "temp_motor": 72.5,
    "temp_atmospheric": 20.3
  }

You can switch to the previous ("old") simulator payload structure with
`--format old`.

Controls: `--rate`, `--workers`, `--duration`, `--count`, and `--format`.
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import random
import ssl
import threading
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any

try:
    import paho.mqtt.client as mqtt
except ImportError as exc:  # pragma: no cover - runtime guidance
    raise SystemExit("Install paho-mqtt first: pip install paho-mqtt") from exc

try:
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import ec
except ImportError:  # pragma: no cover - optional signed mode only
    hashes = serialization = ec = None


DEFAULT_BROKER = "localhost"
DEFAULT_PORT = 8883
DEFAULT_CA_CERT = "mosquitto/certs/ca.crt"
DEFAULT_TOPIC_TEMPLATE = "devices/{device_id}/data"
DEFAULT_TARGET_RATE = 1000.0


@dataclass(frozen=True)
class Config:
    device_id: str
    asset_id: str
    payload_format: str
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
    signed: bool
    private_key_path: Path | None
    seed: int | None
    progress_interval: int
    tls_insecure: bool
    verbose: bool


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
    p = argparse.ArgumentParser(description="Raw telemetry engine (vibration + temps) for MQTT load testing")
    p.add_argument("--device", default="demo_device_001", help="Device ID used in topic and payload")
    p.add_argument("--asset", default="bearing_motor_001", help="Asset ID (used by old format)")
    p.add_argument(
        "--format",
        choices=["new", "old"],
        default="new",
        help="Payload format: 'new' (device_name, scalar vibration/temp) or 'old' (legacy simulator schema)",
    )
    p.add_argument("--rate", type=float, default=DEFAULT_TARGET_RATE, help="Target total messages/sec")
    p.add_argument("--duration", type=float, default=60.0, help="Run duration seconds; 0 means unlimited until --count")
    p.add_argument("--count", type=int, default=0, help="Total messages to send; 0 = no limit")
    p.add_argument("--workers", type=int, default=1, help="Number of concurrent publisher workers")
    p.add_argument("--topic", default="", help="MQTT topic template; defaults to devices/{device_id}/data")
    p.add_argument("--broker", default=DEFAULT_BROKER, help="MQTT broker host")
    p.add_argument("--port", type=int, default=DEFAULT_PORT, help="MQTT broker port")
    p.add_argument("--username", default="pred-device", help="MQTT username")
    p.add_argument("--password", default="dev-device-password", help="MQTT password")
    p.add_argument(
        "--ca-cert",
        default=DEFAULT_CA_CERT,
        help="CA certificate for TLS validation; empty string disables TLS setup",
    )
    p.add_argument("--signed", action="store_true", help="Wrap payload in signed envelope expected by ingestion")
    p.add_argument("--private-key", type=Path, default=None, help="PEM private key path for --signed mode")
    p.add_argument("--seed", type=int, default=None, help="RNG seed for reproducible streams")
    p.add_argument("--progress-interval", type=int, default=1000, help="Print progress every N messages; 0 disables")
    p.add_argument("--tls-insecure", action="store_true", help="Disable TLS cert validation for self-signed brokers")
    p.add_argument("--verbose", "-v", action="store_true", help="Print each message payload to terminal")
    return p.parse_args()


def build_config(args: argparse.Namespace) -> Config:
    if args.rate <= 0:
        raise ValueError("--rate must be > 0")
    if args.workers <= 0:
        raise ValueError("--workers must be > 0")
    if args.count < 0:
        raise ValueError("--count must be >= 0")
    if args.duration < 0:
        raise ValueError("--duration must be >= 0")
    if args.username == "pred-device" and args.workers > 1:
        raise ValueError(
            "--workers > 1 is not allowed with pred-device user in this setup; "
            "broker ACL requires client_id == device_id"
        )

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

    return Config(
        device_id=args.device,
        asset_id=args.asset,
        payload_format=args.format,
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
        signed=args.signed,
        private_key_path=private_key,
        seed=args.seed,
        progress_interval=args.progress_interval,
        tls_insecure=args.tls_insecure,
        verbose=args.verbose,
    )


def create_client(cfg: Config, worker_id: int) -> mqtt.Client:
    if cfg.username == "pred-device" and cfg.workers == 1:
        # Match mosquitto ACL pattern write devices/%c/data where %c is client_id.
        client_id = str(cfg.device_id)
    else:
        client_id = f"raw_telemetry_{cfg.device_id}_{worker_id}"

    client = mqtt.Client(client_id=client_id)
    client.username_pw_set(cfg.username, cfg.password)
    if cfg.ca_cert:
        client.tls_set(ca_certs=cfg.ca_cert, tls_version=ssl.PROTOCOL_TLSv1_2)
        client.tls_insecure_set(cfg.tls_insecure)
    return client


def load_private_key(private_key_path: Path):
    if serialization is None:
        raise RuntimeError(
            "Signed mode requires the cryptography package. "
            "Install it with: pip install cryptography"
        )
    key_text = private_key_path.read_text(encoding="utf-8")
    return serialization.load_pem_private_key(key_text.encode("utf-8"), password=None)


def build_signed_envelope(data_payload: dict[str, Any], sequence: int, private_key) -> str:
    # Sign exactly the bytes that will appear in the data field.
    data_json = json.dumps(data_payload, separators=(",", ":"), sort_keys=False)
    signature = private_key.sign(data_json.encode("utf-8"), ec.ECDSA(hashes.SHA256()))
    signature_b64 = base64.b64encode(signature).decode("utf-8")
    nonce = f"n-{sequence}-{uuid.uuid4().hex}"
    envelope = (
        "{"
        f'"timestamp":{int(time.time())},'
        f'"nonce":"{nonce}",'
        f'"data":{data_json},'
        f'"signature":"{signature_b64}"'
        "}"
    )
    return envelope


def generate_sample(rng: random.Random) -> dict[str, Any]:
    # Base values + small random jitter
    vx = round(rng.normalvariate(0.5, 0.05), 4)  # vibration x RMS-ish
    vy = round(rng.normalvariate(0.48, 0.05), 4)  # vibration y
    temp_motor = round(rng.normalvariate(72.0, 0.8), 3)
    temp_atm = round(rng.normalvariate(21.0, 0.5), 3)
    return {
        "vibration_x": vx,
        "vibration_y": vy,
        "temp_motor": temp_motor,
        "temp_atmospheric": temp_atm,
    }


def build_new_format_payload(cfg: Config, sample: dict[str, Any]) -> dict[str, Any]:
    return {
        "device_name": cfg.device_id,
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "vibration_x": sample["vibration_x"],
        "vibration_y": sample["vibration_y"],
        "temp_motor": sample["temp_motor"],
        "temp_atmospheric": sample["temp_atmospheric"],
    }


def build_old_format_payload(cfg: Config, sample: dict[str, Any], rng: random.Random) -> dict[str, Any]:
    # Match the Go processor's OldSensorEvent struct
    # Fields: device_id, tenant_id, mode, v_rms, temp_c, peak_hz_1, peak_hz_2, peak_hz_3, status
    return {
        "device_id": cfg.device_id,
        "tenant_id": "tenant_001",  # Default tenant for simulation
        "mode": "normal",
        "v_rms": sample["vibration_x"],
        "temp_c": sample["temp_motor"],
        "peak_hz_1": round(rng.uniform(60.0, 120.0), 2),
        "peak_hz_2": round(rng.uniform(180.0, 300.0), 2),
        "peak_hz_3": round(rng.uniform(420.0, 600.0), 2),
        "status": "normal",
    }


def worker_loop(
    worker_id: int,
    cfg: Config,
    sent_counter: AtomicCounter,
    stop_state: StopState,
    started_at: float,
    private_key,
) -> int:
    rng = random.Random((cfg.seed or int(time.time())) + worker_id * 1000)
    client = create_client(cfg, worker_id)
    client.connect(cfg.broker, cfg.port, 60)
    client.loop_start()

    worker_rate = cfg.target_rate / cfg.workers
    interval = 1.0 / worker_rate if worker_rate > 0 else 0.0
    next_tick = time.perf_counter()
    sent = 0

    try:
        while not stop_state.is_set():
            # stop on duration
            if cfg.duration_seconds > 0 and (time.perf_counter() - started_at) >= cfg.duration_seconds:
                stop_state.set()
                break

            # stop on count if provided
            if cfg.count > 0:
                seq = sent_counter.value() + 1
                if seq > cfg.count:
                    stop_state.set()
                    break

            current_seq = sent_counter.increment()
            if cfg.count > 0 and current_seq > cfg.count:
                stop_state.set()
                break

            sample = generate_sample(rng)
            if cfg.payload_format == "old":
                data_payload = build_old_format_payload(cfg, sample, rng)
            else:
                data_payload = build_new_format_payload(cfg, sample)

            if cfg.signed:
                payload = build_signed_envelope(data_payload, current_seq, private_key)
            else:
                payload = json.dumps(data_payload, separators=(",", ":"), sort_keys=False)

            client.publish(cfg.topic, payload, qos=0)
            if cfg.verbose:
                print(f"[worker {worker_id}] {payload}")
            sent += 1

            if cfg.progress_interval > 0 and current_seq % cfg.progress_interval == 0:
                elapsed = max(time.perf_counter() - started_at, 1e-6)
                rate = current_seq / elapsed
                print(f"[worker {worker_id}] sent={current_seq} avg_rate={rate:.1f}/s")

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

    return sent


def main() -> None:
    args = parse_args()
    cfg = build_config(args)

    print("Raw Telemetry Engine")
    print(f"Broker: {cfg.broker}:{cfg.port}")
    print(f"Topic: {cfg.topic}")
    print(f"Device: {cfg.device_id}")
    print(f"Asset: {cfg.asset_id}")
    print(f"Payload format: {cfg.payload_format}")
    print(f"Signed envelope: {cfg.signed}")
    print(f"TLS CA cert: {cfg.ca_cert or 'disabled'}")
    if cfg.private_key_path:
        print(f"Private key: {cfg.private_key_path}")
    print(f"Target rate: {cfg.target_rate} msg/s")
    print(f"Workers: {cfg.workers}")
    print(f"Duration: {cfg.duration_seconds}s")
    print(f"Count limit: {cfg.count or 'unlimited'}")
    print(f"Verbose mode: {cfg.verbose}")

    private_key = None
    if cfg.signed:
        key_path = cfg.private_key_path
        if key_path is None:
            key_env = os.getenv("DEVICE_PRIVATE_KEY")
            if key_env:
                key_path = Path(key_env)
        if key_path is None:
            raise SystemExit("--signed requires --private-key or DEVICE_PRIVATE_KEY")
        private_key = load_private_key(key_path)

    sent_counter = AtomicCounter()
    stop_state = StopState()
    started_at = time.perf_counter()

    threads: list[threading.Thread] = []
    results: list[int] = [0] * cfg.workers

    def run_worker(i: int) -> None:
        results[i] = worker_loop(i + 1, cfg, sent_counter, stop_state, started_at, private_key)

    for i in range(cfg.workers):
        t = threading.Thread(target=run_worker, args=(i,), daemon=True)
        t.start()
        threads.append(t)

    try:
        for t in threads:
            t.join()
    except KeyboardInterrupt:
        stop_state.set()
        print("Stopping workers...")
        for t in threads:
            t.join()

    elapsed = max(time.perf_counter() - started_at, 1e-6)
    total = sent_counter.value()
    print("Finished")
    print(f"Total messages sent: {total}")
    print(f"Average throughput: {total / elapsed:.1f} msg/s")


if __name__ == "__main__":
    main()
