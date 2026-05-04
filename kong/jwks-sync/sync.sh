#!/bin/bash
# Polls Keycloak's JWKS endpoint and pushes Kong's declarative config (with the
# current RS256 signing key substituted in) to Kong's Admin API. Runs forever.
#
# Kong is started in DB-less mode with no declarative config file, so this
# sidecar is the sole source of truth for Kong's runtime configuration.
set -euo pipefail

: "${KEYCLOAK_JWKS_URL:?KEYCLOAK_JWKS_URL must be set}"
: "${KONG_ADMIN_URL:?KONG_ADMIN_URL must be set}"
: "${KONG_CONFIG_TEMPLATE:=/etc/kong/kong.yml}"
: "${POLL_INTERVAL:=300}"
: "${RETRY_INTERVAL:=5}"

log() { echo "[$(date -Iseconds)] $*"; }

# Fetch the JWKS, pick the first RS256 sig key, return it as a PEM public key.
fetch_pubkey() {
  local jwks cert_b64
  jwks=$(curl -fsS --max-time 10 "$KEYCLOAK_JWKS_URL") || return 1

  cert_b64=$(printf '%s' "$jwks" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for k in data.get('keys', []):
    if k.get('alg') == 'RS256' and k.get('x5c'):
        print(k['x5c'][0])
        sys.exit(0)
sys.exit(1)
") || return 1

  printf -- "-----BEGIN CERTIFICATE-----\n%s\n-----END CERTIFICATE-----\n" "$cert_b64" \
    | openssl x509 -pubkey -noout 2>/dev/null
}

push_config() {
  local pubkey="$1"
  local rendered
  rendered=$(PUBKEY="$pubkey" TEMPLATE="$KONG_CONFIG_TEMPLATE" python3 <<'PYEOF'
import os
key = os.environ['PUBKEY'].strip()
indented = '\n'.join('      ' + line for line in key.split('\n'))
with open(os.environ['TEMPLATE']) as f:
    print(f.read().replace('__RSA_PUBLIC_KEY__', indented), end='')
PYEOF
) || return 1

  local tmpfile
  tmpfile=$(mktemp)
  trap 'rm -f "$tmpfile"' RETURN
  printf '%s' "$rendered" > "$tmpfile"
  curl -fsS --max-time 10 -X POST "$KONG_ADMIN_URL/config" \
    -F "config=@$tmpfile" > /dev/null
}

last_pubkey=""

log "Starting JWKS sync loop (poll=${POLL_INTERVAL}s, retry=${RETRY_INTERVAL}s)"
log "  KEYCLOAK_JWKS_URL=$KEYCLOAK_JWKS_URL"
log "  KONG_ADMIN_URL=$KONG_ADMIN_URL"

while true; do
  if ! pubkey=$(fetch_pubkey); then
    log "Failed to fetch JWKS from Keycloak; retrying in ${RETRY_INTERVAL}s"
    sleep "$RETRY_INTERVAL"
    continue
  fi

  if [[ "$pubkey" == "$last_pubkey" ]]; then
    sleep "$POLL_INTERVAL"
    continue
  fi

  if push_config "$pubkey"; then
    last_pubkey="$pubkey"
    log "Pushed updated Kong config (key changed or first run)"
    sleep "$POLL_INTERVAL"
  else
    log "Failed to push config to Kong; retrying in ${RETRY_INTERVAL}s"
    sleep "$RETRY_INTERVAL"
  fi
done
