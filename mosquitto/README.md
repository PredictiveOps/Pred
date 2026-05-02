# Mosquitto Setup Guide

Mosquitto is the MQTT broker for device telemetry in the Pred system. Devices
publish telemetry to Mosquitto over MQTTS, and the ingestion service subscribes
to the same topics before validating payloads and forwarding valid messages to
Kafka.

## Quick Start

```sh
# From repo root
docker compose up -d mosquitto
docker compose logs -f mosquitto
```

Mosquitto will be available at `ssl://localhost:8883`.

See [CONFIGURATION.md](./CONFIGURATION.md) for detailed configuration,
credential override, and troubleshooting notes.

## What gets provisioned

The `mosquitto` service in `docker-compose.yml` starts with
`mosquitto/entrypoint.sh`, which creates a hashed password file at
`/mosquitto/data/passwords` inside the broker's Docker volume.

Two users are created by default:

| User | Default password | Purpose |
|------|------------------|---------|
| `pred-device` | `dev-device-password` | Devices publish telemetry |
| `pred-ingestion` | `dev-ingestion-password` | Ingestion service subscribes |

The ACL file (`mosquitto/acl`) allows:

- `pred-device` to publish to `devices/+/data`
- `pred-ingestion` to subscribe to `devices/+/data`

Mosquitto only handles encrypted transport and topic-level access. Device ID
authentication, payload signature verification, timestamp checks, nonce replay
protection, and tenant/device lookups should happen in the ingestion service.

## Ingestion service config

For local `go run .` from `ingestion-service/`, use:

```env
MQTT_BROKER=ssl://localhost:8883
MQTT_CLIENT_ID=ingestion-service
MQTT_TOPIC=devices/+/data
MQTT_USERNAME=pred-ingestion
MQTT_PASSWORD=dev-ingestion-password
MQTT_CA_CERT=../mosquitto/certs/ca.crt
```

If the ingestion service is later run inside Docker Compose, use
`MQTT_BROKER=ssl://mosquitto:8883` so it connects over the Compose network.

## Overriding the dev passwords

Set these variables in the environment used by Docker Compose, for example in a
top-level `.env` file:

```env
MQTT_DEVICE_USERNAME=pred-device
MQTT_DEVICE_PASSWORD=<device-password>
MQTT_INGESTION_USERNAME=pred-ingestion
MQTT_INGESTION_PASSWORD=<ingestion-password>
```

Then recreate Mosquitto:

```sh
docker compose up -d --force-recreate mosquitto
```

Also update `ingestion-service/.env` so `MQTT_USERNAME` and `MQTT_PASSWORD`
match the ingestion credentials.

## Test with mosquitto clients

Subscribe as the ingestion service:

```sh
docker compose exec mosquitto mosquitto_sub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-ingestion -P dev-ingestion-password \
  -t 'devices/+/data'
```

Publish as a device in another terminal:

```sh
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-device -P dev-device-password \
  -t 'devices/device-001/data' \
  -m '{"temperature":72.4}'
```

## Production checklist

- [ ] Override all default MQTT passwords
- [ ] Keep `allow_anonymous false`
- [ ] Review ACLs before adding new MQTT topics
- [ ] Install production broker TLS certificates under `mosquitto/certs`
- [ ] Store production secrets in a secret manager, not committed files
