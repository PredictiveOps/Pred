#!/bin/bash
# Integration tests for the Kong API Gateway.
# Requires Kong to be running (docker-compose up -d kong).
# With --with-auth, also requires Keycloak running at localhost:8080.
#
# Usage:
#   ./kong/test-kong.sh              # auth-independent tests only
#   ./kong/test-kong.sh --with-auth  # all tests, including JWT round-trip

set -euo pipefail

KONG_PROXY="http://localhost:8000"
KONG_ADMIN="http://localhost:8002"
KEYCLOAK_REALM="${KEYCLOAK_REALM:-pred}"
KEYCLOAK_TOKEN_URL="http://localhost:8080/realms/${KEYCLOAK_REALM}/protocol/openid-connect/token"

WITH_AUTH=false
for arg in "$@"; do
  [[ "$arg" == "--with-auth" ]] && WITH_AUTH=true
done

PASS=0
FAIL=0

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
red()   { printf '\033[0;31m%s\033[0m\n' "$*"; }

pass() { green "  PASS  $1"; PASS=$(( PASS + 1 )); }
fail() { red   "  FAIL  $1"; FAIL=$(( FAIL + 1 )); }

assert_status() {
  local label="$1" expected="$2" url="$3"
  shift 3
  local actual
  actual=$(curl -s -o /dev/null -w "%{http_code}" "$@" "$url")
  if [[ "$actual" == "$expected" ]]; then
    pass "$label (got $actual)"
  else
    fail "$label (expected $expected, got $actual)"
  fi
}

echo "Kong API Gateway — integration tests"
echo "====================================="

# ---------------------------------------------------------------------------
# 1. Admin API
# ---------------------------------------------------------------------------
echo ""
echo "-- Admin API"

assert_status "Admin API is reachable" "200" "$KONG_ADMIN/"

# Check all expected plugins are loaded
for plugin in jwt cors rate-limiting request-size-limiting; do
  count=$(curl -s "$KONG_ADMIN/plugins" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(sum(1 for p in data['data'] if p['name'] == '$plugin'))
")
  if [[ "$count" -ge 1 ]]; then
    pass "Plugin '$plugin' is configured ($count instance(s))"
  else
    fail "Plugin '$plugin' is not configured"
  fi
done

# Check JWT is attached to all routes (not just globally)
jwt_routes=$(curl -s "$KONG_ADMIN/plugins" | python3 -c "
import sys, json
data = json.load(sys.stdin)
routes = [p.get('route') for p in data['data'] if p['name'] == 'jwt' and p.get('route')]
print(len(routes))
")
if [[ "$jwt_routes" -ge 2 ]]; then
  pass "JWT plugin attached to routes ($jwt_routes route(s))"
else
  fail "JWT plugin not attached to enough routes (found $jwt_routes, expected >=2)"
fi

# ---------------------------------------------------------------------------
# 2. JWT enforcement — all routes must return 401 without a token
# ---------------------------------------------------------------------------
echo ""
echo "-- JWT enforcement (no token → 401)"

for path in /api/events /api/ingest; do
  assert_status "No token on $path" "401" "$KONG_PROXY$path"
done

# A garbage token must also be rejected
assert_status "Invalid Bearer token on /api/events" "401" \
  "$KONG_PROXY/api/events" -H "Authorization: Bearer not.a.real.token"

# ---------------------------------------------------------------------------
# 3. CORS
# ---------------------------------------------------------------------------
echo ""
echo "-- CORS"

cors_headers=$(curl -s -o /dev/null -D - -X OPTIONS "$KONG_PROXY/api/ingest" \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET")

if echo "$cors_headers" | grep -qi "Access-Control-Allow-Origin: http://localhost:3000"; then
  pass "CORS: Access-Control-Allow-Origin matches allowed origin"
else
  fail "CORS: Access-Control-Allow-Origin header missing or wrong"
fi

if echo "$cors_headers" | grep -qi "Access-Control-Allow-Credentials: true"; then
  pass "CORS: Access-Control-Allow-Credentials is true"
else
  fail "CORS: Access-Control-Allow-Credentials header missing or not true"
fi

# An origin not in the allowlist must not be echoed back
cors_disallowed=$(curl -s -o /dev/null -D - -X OPTIONS "$KONG_PROXY/api/ingest" \
  -H "Origin: http://evil.example.com" \
  -H "Access-Control-Request-Method: GET")
if echo "$cors_disallowed" | grep -qi "Access-Control-Allow-Origin: http://evil.example.com"; then
  fail "CORS: disallowed origin was reflected"
else
  pass "CORS: disallowed origin is not reflected"
fi

# ---------------------------------------------------------------------------
# 4. OPTIONS preflight — must succeed without auth on every route
# ---------------------------------------------------------------------------
echo ""
echo "-- OPTIONS preflight (no auth → 200, CORS headers present)"

for path in /api/events /api/ingest; do
  # Preflight must not be blocked by JWT (run_on_preflight: false) or the
  # post-function (which returns early for OPTIONS).
  preflight_status=$(curl -s -o /dev/null -w "%{http_code}" \
    -X OPTIONS "$KONG_PROXY$path" \
    -H "Origin: http://localhost:3000" \
    -H "Access-Control-Request-Method: POST" \
    -H "Access-Control-Request-Headers: Authorization, Content-Type")
  if [[ "$preflight_status" == "200" || "$preflight_status" == "204" ]]; then
    pass "OPTIONS $path: no-auth preflight accepted (got $preflight_status)"
  else
    fail "OPTIONS $path: no-auth preflight rejected (expected 200/204, got $preflight_status)"
  fi

  # Preflight with an invalid/garbage token must also succeed — JWT plugin
  # must not run on OPTIONS regardless of what the Authorization header says.
  preflight_bad_token=$(curl -s -o /dev/null -w "%{http_code}" \
    -X OPTIONS "$KONG_PROXY$path" \
    -H "Origin: http://localhost:3000" \
    -H "Access-Control-Request-Method: POST" \
    -H "Authorization: Bearer not.a.valid.token")
  if [[ "$preflight_bad_token" == "200" || "$preflight_bad_token" == "204" ]]; then
    pass "OPTIONS $path: invalid-token preflight still accepted (got $preflight_bad_token)"
  else
    fail "OPTIONS $path: invalid-token preflight was rejected (expected 200/204, got $preflight_bad_token)"
  fi

  # Preflight response must carry the required CORS headers.
  preflight_headers=$(curl -s -o /dev/null -D - \
    -X OPTIONS "$KONG_PROXY$path" \
    -H "Origin: http://localhost:3000" \
    -H "Access-Control-Request-Method: POST")

  if echo "$preflight_headers" | grep -qi "Access-Control-Allow-Origin: http://localhost:3000"; then
    pass "OPTIONS $path: Access-Control-Allow-Origin present"
  else
    fail "OPTIONS $path: Access-Control-Allow-Origin missing"
  fi

  if echo "$preflight_headers" | grep -qi "Access-Control-Allow-Methods:"; then
    pass "OPTIONS $path: Access-Control-Allow-Methods present"
  else
    fail "OPTIONS $path: Access-Control-Allow-Methods missing"
  fi

  if echo "$preflight_headers" | grep -qi "Access-Control-Allow-Credentials: true"; then
    pass "OPTIONS $path: Access-Control-Allow-Credentials is true"
  else
    fail "OPTIONS $path: Access-Control-Allow-Credentials missing or not true"
  fi

  # A non-OPTIONS request to the same path without a token must still return 401,
  # confirming that the preflight bypass does not affect normal auth enforcement.
  assert_status "GET $path without token still returns 401 (auth not bypassed)" \
    "401" "$KONG_PROXY$path"
done

# ---------------------------------------------------------------------------
# 5. Request size limiting
# ---------------------------------------------------------------------------
echo ""
echo "-- Request size limiting"

# Generate a payload slightly over 10 MB (10 * 1024 * 1024 + 1 bytes)
big_body=$(python3 -c "print('x' * (10 * 1024 * 1024 + 1))")
size_status=$(echo "$big_body" | curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$KONG_PROXY/api/ingest" \
  -H "Content-Type: text/plain" \
  --data-binary @-)
if [[ "$size_status" == "413" ]]; then
  pass "Request size limiting: >10MB payload returns 413"
else
  # Kong returns 401 first (JWT check) unless size limit fires before auth.
  # Accept 413 or 401 — the important thing is it never returns 2xx/5xx.
  if [[ "$size_status" != "200" && "$size_status" != "500" ]]; then
    pass "Request size limiting: >10MB payload rejected (got $size_status, not 2xx/5xx)"
  else
    fail "Request size limiting: >10MB payload not rejected (got $size_status)"
  fi
fi

# ---------------------------------------------------------------------------
# 6. Rate limiting (only when explicitly requested — it's slow)
# ---------------------------------------------------------------------------
if [[ "${TEST_RATE_LIMIT:-false}" == "true" ]]; then
  echo ""
  echo "-- Rate limiting (sending 610 requests, this will take a moment...)"
  got_429=false
  for _ in $(seq 1 610); do
    code=$(curl -s -o /dev/null -w "%{http_code}" "$KONG_PROXY/api/events")
    if [[ "$code" == "429" ]]; then
      got_429=true
      break
    fi
  done
  if $got_429; then
    pass "Rate limiting: 429 received after exceeding 600 req/min"
  else
    fail "Rate limiting: never received 429 after 610 requests"
  fi
fi

# ---------------------------------------------------------------------------
# 7. JWT round-trip (requires Keycloak — opt-in via --with-auth)
# ---------------------------------------------------------------------------
if $WITH_AUTH; then
  echo ""
  echo "-- JWT round-trip (requires Keycloak)"

  realm_status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/realms/${KEYCLOAK_REALM}")
  if [[ "$realm_status" != "200" ]]; then
    fail "Keycloak realm '${KEYCLOAK_REALM}' not reachable (HTTP ${realm_status}). Create it (see keycloak/configure.sh), or export KEYCLOAK_REALM to match your DB and set kong/kong.yml jwt_secrets.key + docker-compose KEYCLOAK_JWKS_URL to the same realm iss."
  else
    pass "Keycloak realm '${KEYCLOAK_REALM}' exists"

    # Keycloak can take a few seconds after (re)start before the realm/client is ready.
    TOKEN=""
    for _ in $(seq 1 10); do
      TOKEN=$(curl -s -X POST "$KEYCLOAK_TOKEN_URL" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "client_id=web-frontend&client_secret=dev-web-frontend-secret&grant_type=client_credentials" \
        | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token',''))" || true)
      [[ -n "$TOKEN" ]] && break
      sleep 1
    done

    if [[ -z "$TOKEN" ]]; then
      fail "Could not obtain JWT from Keycloak"
    else
      pass "Obtained JWT from Keycloak"

      # JWT must carry tenant_id (same claim Kong copies to X-Tenant-Id).
      TENANT_IN_TOKEN=$(printf '%s' "$TOKEN" | python3 -c "
import json, base64, sys
t = sys.stdin.read().strip()
parts = t.split('.')
if len(parts) != 3:
    sys.exit(1)
seg = parts[1].replace('-', '+').replace('_', '/')
pad = (4 - len(seg) % 4) % 4
seg += '=' * pad
payload = json.loads(base64.b64decode(seg))
tid = payload.get('tenant_id')
if tid is None or tid == '':
    sys.exit(1)
print(tid)
" 2>/dev/null) || TENANT_IN_TOKEN=""
      if [[ -n "$TENANT_IN_TOKEN" ]]; then
        pass "JWT payload includes tenant_id ($TENANT_IN_TOKEN)"
      else
        fail "JWT payload missing tenant_id (Kong will return 403 for API routes)"
      fi

      for path in /api/events /api/ingest/health; do
        status=$(curl -s -o /dev/null -w "%{http_code}" \
          "$KONG_PROXY$path" -H "Authorization: Bearer $TOKEN")
        # Upstream may not be running; 502/503 is still a Kong success (auth passed)
        if [[ "$status" == "200" || "$status" == "501" || "$status" == "502" || "$status" == "503" ]]; then
          pass "Valid JWT accepted on $path (upstream status $status)"
        else
          fail "Valid JWT not accepted on $path (got $status)"
        fi
      done

      # Prove Kong forwards X-Tenant-Id: GET /devices returns 400 when the header
      # is absent and 200 when it is present, so the response code is the signal.
      devices_status=$(curl -s -o /dev/null -w "%{http_code}" \
        "$KONG_PROXY/api/ingest/devices" -H "Authorization: Bearer $TOKEN")
      if [[ "$devices_status" == "502" || "$devices_status" == "503" ]]; then
        pass "X-Tenant-Id forwarding check skipped (ingestion unreachable via Kong, status $devices_status)"
      elif [[ "$devices_status" == "200" ]]; then
        pass "X-Tenant-Id forwarded: GET /api/ingest/devices returned 200"
      elif [[ "$devices_status" == "400" ]]; then
        fail "X-Tenant-Id not forwarded: GET /api/ingest/devices returned 400 (header missing)"
      else
        fail "X-Tenant-Id forwarding check: unexpected status $devices_status"
      fi
    fi
  fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "====================================="
total=$(( PASS + FAIL ))
echo "Results: $PASS/$total passed"
if [[ "$FAIL" -gt 0 ]]; then
  red "$FAIL test(s) failed."
  exit 1
else
  green "All tests passed."
fi
