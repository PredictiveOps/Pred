# Keycloak Configuration Guide

The realm, client, and a test user are provisioned automatically by the
`keycloak` service in `docker-compose.yml`. Its entrypoint
(`keycloak/entrypoint.sh`) starts Keycloak in the background and runs
`keycloak/configure.sh` against the admin API once it's reachable. The script
is idempotent — re-running it is a no-op.

## What gets provisioned

- **Realm:** `prod-maintenance` (password policy: `length(8) and specialChars(1) and digits(1)`, brute-force protection on)
- **Client scope:** `tenant` with a `tenant_id` user-attribute mapper, attached as a default scope on the client so `tenant_id` is in every token.
- **Client:** `web-frontend` (confidential, standard flow only) with:
  - Redirect URI: `http://localhost:3000/api/auth/callback/keycloak`
  - Web origin: `http://localhost:3000`
  - Secret: `dev-web-frontend-secret` (fixed dev value — see "Overriding the dev secret" below)
- **Test user:** `testuser` / `Test123!`, with attribute `tenant_id=tenant-001`.

## Running it

```sh
docker compose up -d
docker compose logs -f keycloak
```

Look for `[keycloak-config] configuration complete` in the logs. The Keycloak admin UI
is at http://localhost:8080 (`admin` / `changeme`).

## Overriding the dev secret

The default secret (`dev-web-frontend-secret`) is hard-coded into
`web-frontend/.env.example` so a fresh checkout works without manual steps. To
change it:

1. Set `KEYCLOAK_CLIENT_SECRET` in the environment used by `docker compose`
   (e.g. a top-level `.env` file).
2. Set the same value in `web-frontend/.env.local` as `KEYCLOAK_CLIENT_SECRET`.
3. `docker compose up -d --force-recreate keycloak` to re-run the script — it
   will update the existing client's secret.

## Adding more users

Either edit `keycloak/configure.sh` to provision them on every boot, or create
them manually in the admin console at http://localhost:8080.

## Useful Keycloak Endpoints

These are called automatically by NextAuth — no need to call manually:

| Endpoint | Purpose |
|----------|---------|
| `/.well-known/openid-configuration` | OIDC metadata |
| `/protocol/openid-connect/auth` | Authorization request |
| `/protocol/openid-connect/token` | Token exchange |
| `/protocol/openid-connect/userinfo` | Get user info |
| `/protocol/openid-connect/logout` | Logout |

Realm-scoped base URL example:
```
http://localhost:8080/realms/prod-maintenance/.well-known/openid-configuration
```

## Production Checklist

Before deploying to production:

- [ ] Override `KEYCLOAK_ADMIN_PASSWORD` (default `changeme`)
- [ ] Override `KEYCLOAK_CLIENT_SECRET` with a real, secret value
- [ ] Configure SMTP for password-reset emails (admin UI → Realm settings → Email)
- [ ] Use HTTPS (update `KEYCLOAK_URL` / `NEXTAUTH_URL` to `https://`)
- [ ] Use a production-grade Keycloak run mode (not `start-dev`)
- [ ] Configure backup/restore for the Keycloak Postgres database
- [ ] Store secrets in a secret manager, not `.env` files

## Troubleshooting

**`configure.sh` logs an error:** check `docker compose logs keycloak`. The
script retries the admin login for ~2 minutes before giving up. If it fails,
Keycloak itself keeps running so the container stays up — fix the issue and
recreate the container with `docker compose up -d --force-recreate keycloak`.

**Login fails with "Invalid client":** the secret in `web-frontend/.env.local`
doesn't match the secret on the Keycloak client. Re-run with
`docker compose up -d --force-recreate keycloak` after aligning both values.

**Need to start over from scratch:**
```sh
docker compose down -v   # drops the postgres volume, including the keycloak DB
docker compose up -d
```
