# Kong API Gateway

This directory contains the declarative configuration for Kong, which acts as the single entry point (API Gateway) for all frontend API requests.

## Architecture

Kong acts as the single entry point for all frontend API requests, routing traffic securely to the internal microservices running on the Docker network.

- **Proxy Port**: `8000` (Main API endpoint for frontend)
- **Admin Port**: `8002` (Kong Admin API, mapped from internal 8001)

### Services & Routes

Kong uses `strip_path: true` to remove the prefix before forwarding to the upstream service:

| Kong Path              | Upstream Service                | Example                                       |
| ---------------------- | ------------------------------- | --------------------------------------------- |
| `/api/ingest/*`        | `ingestion-service:8003`        | `GET /api/ingest/devices` → `GET /devices`    |
| `/api/events/*`        | `event-processing-service:8001` | `GET /api/events/health` → `GET /health`      |
| `/api/notifications/*` | `notifications-service:8002`    | `POST /api/notifications/send` → `POST /send` |

### DB-less Mode

Kong is configured in **DB-less mode**. Instead of relying on a Postgres database, its entire routing and plugin configuration is loaded directly into memory from the declarative `kong.yml` file. This makes the gateway lightweight and easy to version control.

## Authentication Requirements

**All API endpoints protected by Kong require JWT authentication.**

- **Token Source**: Keycloak (`http://localhost:8080/realms/prod-maintenance`)
- **Token Format**: `Authorization: Bearer <JWT_TOKEN>`
- **Required Claims**: `exp` (expiration time)
- **Algorithm**: RS256 (RSA Public Key from Keycloak)

See the [Verification Guide](#verification-guide) section for instructions on obtaining a test JWT token.

### Plugins Enabled

- **JWT**: Enforces JSON Web Token (JWT) authentication on protected routes. All API endpoints require a valid JWT token from Keycloak (`Authorization: Bearer <token>`). Claims verified: `exp` (expiration).
- **CORS**: Handles Cross-Origin Resource Sharing for safe frontend communication. Configured for `localhost:3000` and `localhost:8000` with credentials enabled.
- **Rate Limiting**: Restricts API calls to 600 requests per minute per IP to prevent abuse and accidental spikes.
- **Request Size Limiting**: Prevents payloads larger than 10MB to protect the ingestion service from DoS attacks.

## Verification Guide

### 1. Check Kong Admin API

```bash
curl http://localhost:8002/
```

You should see a large JSON payload describing the Kong node configuration.

### 2. Verify Routing (JWT Required)

**Note**: All routes require a valid JWT token. Without authentication, you'll receive `401 Unauthorized`.

Test without JWT (should fail):

```bash
curl -i http://localhost:8000/api/events/health
```

Expected: `401 Unauthorized` with `{"message":"Unauthorized"}`

To test with a valid JWT, obtain a token from Keycloak:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/realms/prod-maintenance/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=web-frontend&client_secret=dev-web-frontend-secret&grant_type=client_credentials" | jq -r '.access_token')

# Test with JWT
curl -i http://localhost:8000/api/events/health -H "Authorization: Bearer $TOKEN"
```

Test the ingestion service through Kong:

```bash
curl -i http://localhost:8000/api/ingest/health -H "Authorization: Bearer $TOKEN"
```

Expected: `200 OK`

Test the notifications service through Kong:

```bash
curl -i http://localhost:8000/api/notifications/health -H "Authorization: Bearer $TOKEN"
```

Expected: `200 OK`

### 3. Verify Rate Limiting

Send 610 rapid requests to the gateway:

```bash
for i in {1..610}; do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8000/api/events 2>&1; done
```

You should see `401` responses initially (no JWT), and eventually `429 Too Many Requests` once the 600-request quota is exceeded.

### 4. Verify CORS Headers

```bash
curl -i -X OPTIONS http://localhost:8000/api/ingest \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET"
```

You should see `Access-Control-Allow-Origin: http://localhost:3000` and `Access-Control-Allow-Credentials: true` in the response headers.

## Troubleshooting

### "Unauthorized" (401) responses on all requests

**Cause**: Missing or invalid JWT token in the `Authorization` header.

**Solution**:

1. Verify you're including the `Authorization: Bearer <TOKEN>` header
2. Check that the token is not expired (`exp` claim)
3. Verify the token was issued by the correct Keycloak realm (`prod-maintenance`)
4. Check Kong admin API to verify JWT plugin is attached to routes:
   ```bash
   curl http://localhost:8002/plugins | jq '.data[] | select(.name == "jwt")'
   ```

### JWT Plugin Not Enforcing Authentication

**Cause**: JWT plugin may not be properly attached to routes.

**Solution**:

1. Verify JWT is attached to all routes:
   ```bash
   curl http://localhost:8002/plugins | jq '.data[] | select(.name == "jwt") | {name, route}'
   ```
   Should show 3 entries (one per route)
2. If missing, check the sidecar logs and force a re-push by restarting it:
   ```bash
   docker-compose logs kong-jwks-sync
   docker-compose restart kong-jwks-sync
   ```

### "failure to get a peer from the ring-balancer"

This means Kong's active health checks have marked the upstream targets as unhealthy. Common causes:

1. **Service not running**: Check `docker-compose ps` and `docker-compose logs <service>`.
2. **Stale health state**: Kong caches health state. Fully recreate the container:
   ```bash
   docker-compose rm -f -s kong && docker-compose up -d kong
   ```
   Wait ~12 seconds for health checks to pass (5s interval × 2 successes required).

### Kong won't start

Check logs with `docker-compose logs kong`. Common issues:

- Invalid `kong.yml` syntax (run `docker-compose logs kong` to see error details)
- Custom plugin not installed (stick with bundled plugins: cors, rate-limiting, jwt, request-size-limiting)
- Invalid upstream target format (must be `hostname:port` without protocol prefix)

### CORS Errors (browser blocks requests)

**Cause**: Frontend origin not in CORS allowlist.

**Solution**:

1. Check current CORS configuration:
   ```bash
   curl http://localhost:8002/plugins | jq '.data[] | select(.name == "cors") | .config'
   ```
2. Verify your frontend origin is in the `origins` list (default: `localhost:3000`, `localhost:8000`)
3. If using a different origin, update `kong.yml` and restart the sidecar (`docker-compose restart kong-jwks-sync`) to push the new config to Kong

## Setup & Configuration

### JWKS sidecar (`kong-jwks-sync`)

Kong's bundled `jwt` plugin only accepts a static `rsa_public_key`, so a small
sidecar container keeps that key in sync with Keycloak's JWKS endpoint at
runtime. Source: `kong/jwks-sync/`.

How it works:

1. Kong starts in DB-less mode with **no declarative config file** — its initial
   runtime state is empty (Admin API up, no routes).
2. The `kong-jwks-sync` sidecar polls Keycloak's JWKS endpoint
   (`/realms/prod-maintenance/protocol/openid-connect/certs`) every
   `POLL_INTERVAL` seconds (default 300s).
3. It extracts the RS256 signing key, substitutes `__RSA_PUBLIC_KEY__` in
   `kong/kong.yml` (which is mounted into the sidecar, **not** into Kong), and
   POSTs the rendered config to Kong's Admin API at `/config`. Kong applies it
   atomically without restarting.
4. If Keycloak rotates its signing key, the next poll cycle picks it up and
   pushes a new config — no manual intervention.

On a fresh `docker-compose up`, there's a brief window (typically <10s) where
Kong has no routes configured while the sidecar waits for Keycloak and pushes
the first config. The sidecar retries every `RETRY_INTERVAL` seconds (default
5s) until both Keycloak and Kong are reachable.

`kong/kong.yml` is the **template** — it contains the placeholder
`__RSA_PUBLIC_KEY__` rather than a real key, and is read only by the sidecar.
