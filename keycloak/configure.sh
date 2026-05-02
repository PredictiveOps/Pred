#!/usr/bin/env bash
set -euo pipefail

KC_URL="${KC_URL:-http://localhost:8080}"
KC_ADMIN="${KC_ADMIN:-admin}"
KC_ADMIN_PASSWORD="${KC_ADMIN_PASSWORD:-changeme}"
KC_REALM="${KC_REALM:-prod-maintenance}"
KC_CLIENT_ID="${KC_CLIENT_ID:-web-frontend}"
KC_CLIENT_SECRET="${KC_CLIENT_SECRET:-dev-web-frontend-secret}"
KC_REDIRECT_URI="${KC_REDIRECT_URI:-http://localhost:3000/api/auth/callback/keycloak}"
KC_WEB_ORIGIN="${KC_WEB_ORIGIN:-http://localhost:3000}"
TEST_USERNAME="${TEST_USERNAME:-testuser}"
TEST_USER_EMAIL="${TEST_USER_EMAIL:-test@example.com}"
TEST_USER_PASSWORD="${TEST_USER_PASSWORD:-Test123!}"
TEST_USER_TENANT="${TEST_USER_TENANT:-tenant-001}"

KCADM=/opt/keycloak/bin/kcadm.sh

log() { echo "[keycloak-config] $*"; }

log "waiting for keycloak admin login at ${KC_URL}"
for i in $(seq 1 60); do
  if "$KCADM" config credentials \
      --server "$KC_URL" \
      --realm master \
      --user "$KC_ADMIN" \
      --password "$KC_ADMIN_PASSWORD" >/dev/null 2>&1; then
    log "admin login ok"
    break
  fi
  if [ "$i" -eq 60 ]; then
    log "timed out waiting for keycloak"
    exit 1
  fi
  sleep 2
done

# --- realm ---
if "$KCADM" get "realms/${KC_REALM}" >/dev/null 2>&1; then
  log "realm '${KC_REALM}' already exists"
else
  log "creating realm '${KC_REALM}'"
  "$KCADM" create realms \
    -s "realm=${KC_REALM}" \
    -s enabled=true \
    -s sslRequired=external \
    -s registrationAllowed=false \
    -s loginWithEmailAllowed=true \
    -s duplicateEmailsAllowed=false \
    -s resetPasswordAllowed=true \
    -s editUsernameAllowed=false \
    -s bruteForceProtected=true \
    -s 'passwordPolicy=length(8) and specialChars(1) and digits(1)'
fi

# --- tenant client scope ---
TENANT_SCOPE_ID=$("$KCADM" get client-scopes -r "$KC_REALM" \
  --query 'first=0' --query 'max=200' \
  --fields id,name --format csv --noquotes 2>/dev/null \
  | awk -F, '$2 == "tenant" { print $1 }' || true)

if [ -z "${TENANT_SCOPE_ID:-}" ]; then
  log "creating client scope 'tenant'"
  TENANT_SCOPE_ID=$("$KCADM" create client-scopes -r "$KC_REALM" \
    -s name=tenant \
    -s protocol=openid-connect \
    -s 'attributes."include.in.token.scope"=true' \
    -i)
  "$KCADM" create "client-scopes/${TENANT_SCOPE_ID}/protocol-mappers/models" \
    -r "$KC_REALM" \
    -s name=tenant_id \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-usermodel-attribute-mapper \
    -s consentRequired=false \
    -s 'config."user.attribute"=tenant_id' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true' \
    -s 'config."claim.name"=tenant_id' \
    -s 'config."jsonType.label"=String' \
    -s 'config."userinfo.token.claim"=true'
else
  log "client scope 'tenant' already exists (${TENANT_SCOPE_ID})"
fi

# --- client ---
CLIENT_UUID=$("$KCADM" get clients -r "$KC_REALM" \
  --query "clientId=${KC_CLIENT_ID}" --fields id --format csv --noquotes 2>/dev/null \
  | tail -n +1 | head -n 1 || true)

if [ -z "${CLIENT_UUID:-}" ]; then
  log "creating client '${KC_CLIENT_ID}'"
  CLIENT_UUID=$("$KCADM" create clients -r "$KC_REALM" \
    -s "clientId=${KC_CLIENT_ID}" \
    -s 'name=Pred Web Frontend' \
    -s protocol=openid-connect \
    -s publicClient=false \
    -s clientAuthenticatorType=client-secret \
    -s enabled=true \
    -s standardFlowEnabled=true \
    -s implicitFlowEnabled=false \
    -s directAccessGrantsEnabled=false \
    -s serviceAccountsEnabled=false \
    -s authorizationServicesEnabled=false \
    -s frontchannelLogout=true \
    -s "redirectUris=[\"${KC_REDIRECT_URI}\"]" \
    -s "webOrigins=[\"${KC_WEB_ORIGIN}\"]" \
    -i)
else
  log "client '${KC_CLIENT_ID}' already exists (${CLIENT_UUID}); updating"
fi

"$KCADM" update "clients/${CLIENT_UUID}" -r "$KC_REALM" \
  -s publicClient=false \
  -s clientAuthenticatorType=client-secret \
  -s enabled=true \
  -s standardFlowEnabled=true \
  -s implicitFlowEnabled=false \
  -s directAccessGrantsEnabled=false \
  -s serviceAccountsEnabled=false \
  -s frontchannelLogout=true \
  -s "secret=${KC_CLIENT_SECRET}" \
  -s "redirectUris=[\"${KC_REDIRECT_URI}\"]" \
  -s "webOrigins=[\"${KC_WEB_ORIGIN}\"]"

# Attach tenant scope as default so tenant_id is in the token without opting in.
"$KCADM" update "clients/${CLIENT_UUID}/default-client-scopes/${TENANT_SCOPE_ID}" \
  -r "$KC_REALM" >/dev/null 2>&1 || true

# --- test user ---
USER_ID=$("$KCADM" get users -r "$KC_REALM" \
  --query "username=${TEST_USERNAME}" --query 'exact=true' \
  --fields id --format csv --noquotes 2>/dev/null \
  | tail -n +1 | head -n 1 || true)

if [ -z "${USER_ID:-}" ]; then
  log "creating user '${TEST_USERNAME}'"
  USER_ID=$("$KCADM" create users -r "$KC_REALM" \
    -s "username=${TEST_USERNAME}" \
    -s "email=${TEST_USER_EMAIL}" \
    -s firstName=Test \
    -s lastName=User \
    -s enabled=true \
    -s emailVerified=true \
    -s "attributes.tenant_id=[\"${TEST_USER_TENANT}\"]" \
    -i)
  "$KCADM" set-password -r "$KC_REALM" --userid "$USER_ID" \
    --new-password "$TEST_USER_PASSWORD"
else
  log "user '${TEST_USERNAME}' already exists (${USER_ID})"
fi

log "configuration complete: realm=${KC_REALM} client=${KC_CLIENT_ID} user=${TEST_USERNAME}"
