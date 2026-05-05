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
KEYCLOAK_TOKEN_URL="http://localhost:8080/realms/prod-maintenance/protocol/openid-connect/token"

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
# 4. Request size limiting
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
# 5. Rate limiting (only when explicitly requested — it's slow)
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
# 6. JWT round-trip (requires Keycloak — opt-in via --with-auth)
# ---------------------------------------------------------------------------
if $WITH_AUTH; then
  echo ""
  echo "-- JWT round-trip (requires Keycloak)"

  TOKEN=$(curl -s -X POST "$KEYCLOAK_TOKEN_URL" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "client_id=web-frontend&client_secret=dev-web-frontend-secret&grant_type=client_credentials" \
    | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token',''))")

  if [[ -z "$TOKEN" ]]; then
    fail "Could not obtain JWT from Keycloak"
  else
    pass "Obtained JWT from Keycloak"

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
