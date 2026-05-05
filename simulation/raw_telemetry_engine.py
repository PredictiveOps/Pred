#!/usr/bin/env python3
"""Simple raw telemetry engine publishing per-sample vibration + temps over MQTT.

Publishes JSON messages with the structure:
  {
    "device_name": "demo_device_001",
    "timestamp": "2026-05-06T12:00:00Z",
    "vibration_x": 0.123,
    "vibration_y": 0.456,
    "temp_motor": 72.5,
    "temp_atmospheric": 20.3
  }

Controls: `--rate`, `--workers`, `--duration`, `--count`, and `--delay`.
Designed for easy integration with the existing mosquitto/ingestion pipeline.
"""

from __future__ import annotations

import argparse
import json
import random
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

try:
    import paho.mqtt.client as mqtt
except ImportError as exc:  # pragma: no cover - runtime guidance
    raise SystemExit("Install paho-mqtt first: pip install paho-mqtt") from exc


DEFAULT_BROKER = "localhost"
DEFAULT_PORT = 8883
DEFAULT_TOPIC_TEMPLATE = "devices/{device_id}/data"
DEFAULT_TARGET_RATE = 1000.0


@dataclass(frozen=True)
class Config:
    device_id: str
    target_rate: float
    duration_seconds: float
    count: int
    workers: int
    topic: str
    broker: str
    port: int
    username: str
    password: str
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
    p = argparse.ArgumentParser(description="Raw telemetry engine (vibration + temps) for MQTT load testing")
    p.add_argument("--device", default="demo_device_001", help="Device ID used in topic and payload")
    p.add_argument("--rate", type=float, default=DEFAULT_TARGET_RATE, help="Target total messages/sec")
    p.add_argument("--duration", type=float, default=60.0, help="Run duration seconds; 0 means unlimited until --count")
    p.add_argument("--count", type=int, default=0, help="Total messages to send; 0 = no limit")
    p.add_argument("--workers", type=int, default=1, help="Number of concurrent publisher workers")
    p.add_argument("--topic", default="", help="MQTT topic template; defaults to devices/{device_id}/data")
    p.add_argument("--broker", default=DEFAULT_BROKER, help="MQTT broker host")
    p.add_argument("--port", type=int, default=DEFAULT_PORT, help="MQTT broker port")
    p.add_argument("--username", default="pred-device", help="MQTT username")
    p.add_argument("--password", default="dev-device-password", help="MQTT password")
    p.add_argument("--seed", type=int, default=None, help="RNG seed for reproducible streams")
    p.add_argument("--progress-interval", type=int, default=1000, help="Print progress every N messages; 0 disables")
    p.add_argument("--tls-insecure", action="store_true", help="Disable TLS cert validation for self-signed brokers")
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

    project_root = Path(__file__).resolve().parents[1]
    topic = args.topic or DEFAULT_TOPIC_TEMPLATE.format(device_id=args.device)

    return Config(
        device_id=args.device,
        target_rate=args.rate,
        duration_seconds=args.duration,
        count=args.count,
        workers=args.workers,
        topic=topic,
        broker=args.broker,
        port=args.port,
        username=args.username,
        password=args.password,
        seed=args.seed,
        progress_interval=args.progress_interval,
        tls_insecure=args.tls_insecure,
    )


def create_client(cfg: Config, worker_id: int) -> mqtt.Client:
    client = mqtt.Client(client_id=f"raw_telemetry_{cfg.device_id}_{worker_id}")
    client.username_pw_set(cfg.username, cfg.password)
    # TLS setup is intentionally minimal; broker-side certs expected in compose.
    # If using TLS with self-signed certs, the mosquitto service uses CA certs.
    return client


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


def worker_loop(worker_id: int, cfg: Config, sent_counter: AtomicCounter, stop_state: StopState, started_at: float) -> int:
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
            payload = {
                "device_name": cfg.device_id,
                "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                **sample,
            }

            client.publish(cfg.topic, json.dumps(payload), qos=0)
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
    print(f"Target rate: {cfg.target_rate} msg/s")
    print(f"Workers: {cfg.workers}")
    print(f"Duration: {cfg.duration_seconds}s")
    print(f"Count limit: {cfg.count or 'unlimited'}")

    sent_counter = AtomicCounter()
    stop_state = StopState()
    started_at = time.perf_counter()

    threads: list[threading.Thread] = []
    results: list[int] = [0] * cfg.workers

    def run_worker(i: int) -> None:
        results[i] = worker_loop(i + 1, cfg, sent_counter, stop_state, started_at)

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
