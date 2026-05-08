#!/bin/bash
set -e

echo "[ML Service] Waiting for PostgreSQL at postgres:5432..."

# Wait for postgres to be ready
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
        echo "[ML Service] PostgreSQL is ready!"
        break
    fi
    echo "[ML Service] Waiting for PostgreSQL... ($i/30)"
    sleep 2
done

echo "[ML Service] Starting API server..."
exec python -m uvicorn prediction_api:app --host 0.0.0.0 --port 8000
