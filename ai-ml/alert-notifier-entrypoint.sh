#!/bin/bash
set -e

echo "[Alert Notifier] Waiting for Kafka at kafka:29092..."

# Wait for Kafka to be ready
for i in {1..30}; do
    if python -c "
import socket
import sys
try:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(2)
    result = sock.connect_ex(('kafka', 29092))
    sock.close()
    sys.exit(0 if result == 0 else 1)
except Exception:
    sys.exit(1)
" 2>/dev/null; then
        echo "[Alert Notifier] Kafka is ready!"
        break
    fi
    echo "[Alert Notifier] Waiting for Kafka... ($i/30)"
    sleep 2
done

echo "[Alert Notifier] Starting alert notifier service..."
exec python -m alert_notifier
