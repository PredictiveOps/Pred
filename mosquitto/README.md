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

## Generate dev certificates (for local testing)

Run these from the repo root; they create a CA (used only to sign the server cert) and a server certificate under `./mosquitto/certs`.

```sh
# create cert dir and cd into it
mkdir -p ./mosquitto/certs && cd ./mosquitto/certs

# Create a local CA (you can archive/remove ca.key after signing)
openssl genpkey -algorithm RSA -out ca.key -pkeyopt rsa_keygen_bits:4096
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt -subj "/CN=Pred Local CA"

# Create server key and CSR
openssl genpkey -algorithm RSA -out server.key -pkeyopt rsa_keygen_bits:2048
openssl req -new -key server.key -out server.csr -subj "/CN=localhost"

# Create SAN extfile and sign the server CSR with the CA
cat > server.ext <<'EOF'
[req]
distinguished_name = req_distinguished_name
req_extensions = req_ext
prompt = no

[req_distinguished_name]
CN = localhost

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF

openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt -days 825 -sha256 -extfile server.ext

chmod 600 server.key
chmod 644 server.crt ca.crt
```


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
- [ ] **Disable `InsecureSkipVerify` in ingestion service** (set to `false` in `services/MQTT.service.go`)
- [ ] Generate certificates with proper SANs covering all broker hostnames/IPs

**⚠️ IMPORTANT - TLS Certificate Verification:**
The current ingestion service configuration uses `InsecureSkipVerify: true` which disables TLS certificate validation. This is acceptable for local development but **must be disabled in production**.

Before deploying to production:
1. Generate proper TLS certificates with SANs covering your broker's hostname(s)
2. Set `InsecureSkipVerify: false` in `ingestion-service/services/MQTT.service.go`
3. Configure all clients to verify the broker's certificate against a trusted CA
