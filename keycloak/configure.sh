#!/usr/bin/env bash
set -euo pipefail

KC_URL="${KC_URL:-http://localhost:8080}"
KC_ADMIN="${KC_ADMIN:-admin}"
KC_ADMIN_PASSWORD="${KC_ADMIN_PASSWORD:-changeme}"
KC_REALM="${KC_REALM:-pred}"
KC_CLIENT_ID="${KC_CLIENT_ID:-web-frontend}"
KC_CLIENT_SECRET="${KC_CLIENT_SECRET:-dev-web-frontend-secret}"
KC_REDIRECT_URI="${KC_REDIRECT_URI:-http://localhost:3000/api/auth/callback/keycloak}"
KC_WEB_ORIGIN="${KC_WEB_ORIGIN:-http://localhost:3000}"
KC_NOTIFICATIONS_CLIENT_ID="${KC_NOTIFICATIONS_CLIENT_ID:-notifications-service}"
KC_NOTIFICATIONS_CLIENT_SECRET="${KC_NOTIFICATIONS_CLIENT_SECRET:-dev-notifications-service-secret}"
TEST_USERNAME="${TEST_USERNAME:-testuser}"
TEST_USER_EMAIL="${TEST_USER_EMAIL:-test@example.com}"
TEST_USER_PASSWORD="${TEST_USER_PASSWORD:-Test123!}"
TEST_USER_TENANT="${TEST_USER_TENANT:-tenant-001}"
SERVICE_ACCOUNT_TENANT="${SERVICE_ACCOUNT_TENANT:-tenant-001}"

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

# --- realm user profile: allow unmanaged attributes ---
"$KCADM" update "users/profile" -r "$KC_REALM" \
  -s 'unmanagedAttributePolicy=ENABLED'

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
    -s serviceAccountsEnabled=true \
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
  -s serviceAccountsEnabled=true \
  -s frontchannelLogout=true \
  -s "secret=${KC_CLIENT_SECRET}" \
  -s "redirectUris=[\"${KC_REDIRECT_URI}\"]" \
  -s "webOrigins=[\"${KC_WEB_ORIGIN}\"]"

# --- tenant_id protocol mapper on client ---
MAPPER_ID=$("$KCADM" get "clients/${CLIENT_UUID}/protocol-mappers/models" -r "$KC_REALM" \
  --fields id,name --format csv --noquotes 2>/dev/null \
  | grep ',tenant_id$' | cut -d, -f1 || true)

if [ -z "${MAPPER_ID:-}" ]; then
  log "creating 'tenant_id' protocol mapper on client '${KC_CLIENT_ID}'"
  "$KCADM" create "clients/${CLIENT_UUID}/protocol-mappers/models" -r "$KC_REALM" \
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
  log "'tenant_id' protocol mapper already exists on client (${MAPPER_ID})"
fi

# Ensure the client-credentials token (service account) also contains tenant_id.
# Keycloak models the service account as a user; we set the user attribute that
# the mapper reads from.
SERVICE_ACCOUNT_USER_ID=$("$KCADM" get "clients/${CLIENT_UUID}/service-account-user" -r "$KC_REALM" \
  --fields id --format csv --noquotes 2>/dev/null | tail -n +1 | head -n 1 || true)

if [ -n "${SERVICE_ACCOUNT_USER_ID:-}" ]; then
  log "setting service-account tenant_id=${SERVICE_ACCOUNT_TENANT}"
  "$KCADM" update "users/${SERVICE_ACCOUNT_USER_ID}" -r "$KC_REALM" \
    -s "attributes.tenant_id=[\"${SERVICE_ACCOUNT_TENANT}\"]" >/dev/null
else
  log "could not locate service-account user for client '${KC_CLIENT_ID}'"
fi

# --- notifications-service client ---
NOTIFICATIONS_CLIENT_UUID=$("$KCADM" get clients -r "$KC_REALM" \
  --query "clientId=${KC_NOTIFICATIONS_CLIENT_ID}" --fields id --format csv --noquotes 2>/dev/null \
  | tail -n +1 | head -n 1 || true)

if [ -z "${NOTIFICATIONS_CLIENT_UUID:-}" ]; then
  log "creating notifications-service client '${KC_NOTIFICATIONS_CLIENT_ID}'"
  NOTIFICATIONS_CLIENT_UUID=$("$KCADM" create clients -r "$KC_REALM" \
    -s "clientId=${KC_NOTIFICATIONS_CLIENT_ID}" \
    -s 'name=Notifications Service' \
    -s protocol=openid-connect \
    -s publicClient=false \
    -s clientAuthenticatorType=client-secret \
    -s enabled=true \
    -s standardFlowEnabled=false \
    -s implicitFlowEnabled=false \
    -s directAccessGrantsEnabled=false \
    -s serviceAccountsEnabled=true \
    -s authorizationServicesEnabled=false \
    -s frontchannelLogout=false \
    -s "secret=${KC_NOTIFICATIONS_CLIENT_SECRET}" \
    -i)
else
  log "notifications-service client '${KC_NOTIFICATIONS_CLIENT_ID}' already exists (${NOTIFICATIONS_CLIENT_UUID}); updating"
fi

"$KCADM" update "clients/${NOTIFICATIONS_CLIENT_UUID}" -r "$KC_REALM" \
  -s publicClient=false \
  -s clientAuthenticatorType=client-secret \
  -s enabled=true \
  -s standardFlowEnabled=false \
  -s implicitFlowEnabled=false \
  -s directAccessGrantsEnabled=false \
  -s serviceAccountsEnabled=true \
  -s frontchannelLogout=false \
  -s "secret=${KC_NOTIFICATIONS_CLIENT_SECRET}"

# --- tenant_id protocol mapper on notifications-service client ---
NOTIFICATIONS_MAPPER_ID=$("$KCADM" get "clients/${NOTIFICATIONS_CLIENT_UUID}/protocol-mappers/models" -r "$KC_REALM" \
  --fields id,name --format csv --noquotes 2>/dev/null \
  | grep ',tenant_id$' | cut -d, -f1 || true)

if [ -z "${NOTIFICATIONS_MAPPER_ID:-}" ]; then
  log "creating 'tenant_id' protocol mapper on notifications-service client '${KC_NOTIFICATIONS_CLIENT_ID}'"
  "$KCADM" create "clients/${NOTIFICATIONS_CLIENT_UUID}/protocol-mappers/models" -r "$KC_REALM" \
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
  log "'tenant_id' protocol mapper already exists on notifications-service client (${NOTIFICATIONS_MAPPER_ID})"
fi

# Set service account tenant_id for notifications-service
NOTIFICATIONS_SERVICE_ACCOUNT_USER_ID=$("$KCADM" get "clients/${NOTIFICATIONS_CLIENT_UUID}/service-account-user" -r "$KC_REALM" \
  --fields id --format csv --noquotes 2>/dev/null | tail -n +1 | head -n 1 || true)

if [ -n "${NOTIFICATIONS_SERVICE_ACCOUNT_USER_ID:-}" ]; then
  log "setting notifications-service account tenant_id=${SERVICE_ACCOUNT_TENANT}"
  "$KCADM" update "users/${NOTIFICATIONS_SERVICE_ACCOUNT_USER_ID}" -r "$KC_REALM" \
    -s "attributes.tenant_id=[\"${SERVICE_ACCOUNT_TENANT}\"]" >/dev/null
else
  log "could not locate service-account user for notifications-service client '${KC_NOTIFICATIONS_CLIENT_ID}'"
fi

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

log "configuration complete: realm=${KC_REALM} client=${KC_CLIENT_ID} notifications-client=${KC_NOTIFICATIONS_CLIENT_ID} user=${TEST_USERNAME}"
