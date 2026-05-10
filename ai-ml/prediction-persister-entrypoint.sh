#!/bin/bash
set -e

echo "[Prediction Persister] Waiting for PostgreSQL at postgres:5432..."

# Wait for PostgreSQL to be ready
for i in {1..30}; do
    if python -c "
import socket
import sys
try:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(2)
    result = sock.connect_ex(('postgres', 5432))
    sock.close()
    sys.exit(0 if result == 0 else 1)
except Exception:
    sys.exit(1)
" 2>/dev/null; then
        echo "[Prediction Persister] PostgreSQL is ready!"
        break
    fi
    echo "[Prediction Persister] Waiting for PostgreSQL... ($i/30)"
    sleep 2
done

echo "[Prediction Persister] Waiting for Kafka at kafka:9092..."

# Wait for Kafka to be ready
for i in {1..30}; do
    if python -c "
import socket
import sys
try:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(2)
    result = sock.connect_ex(('kafka', 9092))
    sock.close()
    sys.exit(0 if result == 0 else 1)
except Exception:
    sys.exit(1)
" 2>/dev/null; then
        echo "[Prediction Persister] Kafka is ready!"
        break
    fi
    echo "[Prediction Persister] Waiting for Kafka... ($i/30)"
    sleep 2
done

echo "[Prediction Persister] Starting prediction persistence service..."
exec python -m prediction_persister
