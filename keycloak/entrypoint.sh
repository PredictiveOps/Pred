#!/usr/bin/env bash
set -euo pipefail

/opt/keycloak/bin/kc.sh start-dev --metrics-enabled=true &
KC_PID=$!

trap 'kill -TERM "$KC_PID" 2>/dev/null || true; wait "$KC_PID"' TERM INT

if /bin/bash /opt/keycloak-config/configure.sh; then
  :
else
  echo "[keycloak-config] configure.sh failed; keycloak is still running"
fi

wait "$KC_PID"
