#!/bin/sh
set -eu

DEVICE_USERNAME="${MQTT_DEVICE_USERNAME:-pred-device}"
DEVICE_PASSWORD="${MQTT_DEVICE_PASSWORD:-dev-device-password}"
INGESTION_USERNAME="${MQTT_INGESTION_USERNAME:-pred-ingestion}"
INGESTION_PASSWORD="${MQTT_INGESTION_PASSWORD:-dev-ingestion-password}"
PASSWORD_FILE="/mosquitto/data/passwords"
CA_FILE="/mosquitto/config/certs/ca.crt"
CERT_FILE="/mosquitto/config/certs/server.crt"
KEY_FILE="/mosquitto/config/certs/server.key"
RUNTIME_CERT_DIR="/mosquitto/data/certs"
RUNTIME_CA_FILE="$RUNTIME_CERT_DIR/ca.crt"
RUNTIME_CERT_FILE="$RUNTIME_CERT_DIR/server.crt"
RUNTIME_KEY_FILE="$RUNTIME_CERT_DIR/server.key"

if [ -z "$DEVICE_USERNAME" ] || [ -z "$DEVICE_PASSWORD" ] || [ -z "$INGESTION_USERNAME" ] || [ -z "$INGESTION_PASSWORD" ]; then
  echo "[mosquitto-config] MQTT usernames and passwords must not be empty" >&2
  exit 1
fi

for cert_file in "$CA_FILE" "$CERT_FILE" "$KEY_FILE"; do
  if [ ! -f "$cert_file" ]; then
    echo "[mosquitto-config] missing TLS file: $cert_file" >&2
    echo "[mosquitto-config] add broker certificates under mosquitto/certs before starting MQTTS" >&2
    exit 1
  fi
done

echo "[mosquitto-config] preparing runtime TLS files"
mkdir -p "$RUNTIME_CERT_DIR"
cp "$CA_FILE" "$RUNTIME_CA_FILE"
cp "$CERT_FILE" "$RUNTIME_CERT_FILE"
cp "$KEY_FILE" "$RUNTIME_KEY_FILE"
chown mosquitto:mosquitto "$RUNTIME_CA_FILE" "$RUNTIME_CERT_FILE" "$RUNTIME_KEY_FILE"
chmod 0644 "$RUNTIME_CA_FILE" "$RUNTIME_CERT_FILE"
chmod 0600 "$RUNTIME_KEY_FILE"

echo "[mosquitto-config] writing password file"
rm -f "$PASSWORD_FILE"
mosquitto_passwd -b -c "$PASSWORD_FILE" "$DEVICE_USERNAME" "$DEVICE_PASSWORD"
mosquitto_passwd -b "$PASSWORD_FILE" "$INGESTION_USERNAME" "$INGESTION_PASSWORD"
chmod 0600 "$PASSWORD_FILE"
chown mosquitto:mosquitto "$PASSWORD_FILE"

echo "[mosquitto-config] starting broker"
exec mosquitto -c /mosquitto/config/mosquitto.conf
