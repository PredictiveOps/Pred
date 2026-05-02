# Mosquitto Configuration Guide

The MQTT broker is provisioned automatically by the `mosquitto` service in
`docker-compose.yml`. Its entrypoint (`mosquitto/entrypoint.sh`) checks that
TLS certificates exist, creates the password database from environment
variables, then starts Mosquitto with `mosquitto/mosquitto.conf`.

Mosquitto is responsible for encrypted transport, username/password
authentication, and topic ACLs. It does not validate device IDs, payload
signatures, timestamps, or nonces. Those checks belong in the ingestion service
because ingestion can use the server database and Redis/device-key cache.

## What gets configured

- **Broker listener:** `0.0.0.0:8883` using MQTTS
- **TLS files:** `mosquitto/certs/ca.crt`, `server.crt`, and `server.key`
- **Anonymous access:** disabled
- **Password database:** generated at `/mosquitto/data/passwords` inside the
  `mosquitto_data` Docker volume
- **ACL file:** `mosquitto/acl`
- **Device user:** `pred-device` / `dev-device-password`
- **Ingestion user:** `pred-ingestion` / `dev-ingestion-password`

## Access rules

| User | Access |
|------|--------|
| `pred-device` | Publish to `devices/+/data` |
| `pred-ingestion` | Subscribe to `devices/+/data` |

## Running it

```sh
docker compose up -d mosquitto
docker compose logs -f mosquitto
```

The broker is available at:

```text
ssl://localhost:8883
```

The container will not start until these files exist:

```text
mosquitto/certs/ca.crt
mosquitto/certs/server.crt
mosquitto/certs/server.key
```

If you use the same certificate authority and certificate process as HTTPS,
make sure the broker certificate includes the MQTT hostname in its SAN list,
for example `localhost`, `mosquitto`, or your production MQTT domain.

## Overriding the dev credentials

Set these values in the environment used by Docker Compose, for example a
top-level `.env` file:

```env
MQTT_DEVICE_USERNAME=pred-device
MQTT_DEVICE_PASSWORD=<device-password>
MQTT_INGESTION_USERNAME=pred-ingestion
MQTT_INGESTION_PASSWORD=<ingestion-password>
```

Then recreate the broker:

```sh
docker compose up -d --force-recreate mosquitto
```

Update `ingestion-service/.env` with the ingestion credential:

```env
MQTT_BROKER=ssl://localhost:8883
MQTT_CA_CERT=../mosquitto/certs/ca.crt
MQTT_USERNAME=pred-ingestion
MQTT_PASSWORD=<ingestion-password>
```

## Testing credentials

Subscribe as the ingestion service:

```sh
docker compose exec mosquitto mosquitto_sub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-ingestion -P dev-ingestion-password \
  -t 'devices/+/data'
```

Publish as a device:

```sh
docker compose exec mosquitto mosquitto_pub \
  -h localhost -p 8883 \
  --cafile /mosquitto/config/certs/ca.crt \
  -u pred-device -P dev-device-password \
  -t 'devices/device-001/data' \
  -m '{"temperature":72.4}'
```

## Troubleshooting

**Connection refused:** check that `docker compose ps mosquitto` shows the
container as healthy and that port `8883` is not already in use.

**Authentication failed:** recreate the broker after changing credential
environment variables. The password database is regenerated on container start.

**Certificate verification failed:** confirm the ingestion service has
`MQTT_CA_CERT` pointing at the CA that signed the broker certificate, and that
the certificate SAN matches the hostname in `MQTT_BROKER`.

**Ingestion service cannot connect:** use `ssl://localhost:8883` when running
the ingestion service locally with `go run .`. Use `ssl://mosquitto:8883` only
when the ingestion service runs inside Docker Compose.
