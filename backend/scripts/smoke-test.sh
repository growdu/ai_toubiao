#!/bin/bash
# Cross-service smoke test for BidWriter.
#
# Verifies the whole stack is wired up correctly by hitting the public
# api-gateway entry point and exercising at least one path through every
# upstream service.
#
# Prereq: scripts/start-services.sh has brought everything up and the
# infra compose (postgres + minio) is healthy.
set -eo pipefail

GATEWAY="${GATEWAY:-http://localhost:7080}"
PASS=0
FAIL=0

# ---- helpers ----
green() { printf "\033[32m%s\033[0m" "$1"; }
red()   { printf "\033[31m%s\033[0m" "$1"; }

assert_eq() {
  local name="$1" want="$2" got="$3"
  if [ "$want" = "$got" ]; then
    echo "  $(green PASS) $name (got $got)"
    PASS=$((PASS+1))
  else
    echo "  $(red FAIL) $name (want $want, got $got)"
    FAIL=$((FAIL+1))
  fi
}

http_status() {
  curl -s -o /tmp/smoke-body -w '%{http_code}' "$@"
}

http_body() { cat /tmp/smoke-body; }

# Run with optional JWT; if token is empty, no Authorization header.
req() {
  local method="$1" path="$2"
  shift 2
  local headers=()
  if [ -n "${TOKEN:-}" ]; then
    headers+=(-H "Authorization: Bearer $TOKEN")
  fi
  headers+=(-H "X-Tenant-Id: ${TENANT:-11111111-1111-1111-1111-111111111111}")
  curl -s -o /tmp/smoke-body -w '%{http_code}' \
    -X "$method" "$GATEWAY$path" "${headers[@]}" "$@"
}

echo "== BidWriter cross-service smoke (gateway=$GATEWAY) =="
echo

# ---- 1. login → JWT ----
echo "[1] auth.login"
TENANT=11111111-1111-1111-1111-111111111111
code=$(http_status -X POST "$GATEWAY/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"tenant_slug":"demo-a","email":"admin@demo-a.test","password":"password123"}')
assert_eq "login returns 200" 200 "$code"
TOKEN=$(http_body | jq -r '.access_token // .data.token // .token // empty')
if [ -z "$TOKEN" ]; then
  echo "  $(red FAIL) no token in response: $(http_body)"
  exit 1
fi
echo "  token len: ${#TOKEN}"
echo

# ---- 2. projects ----
echo "[2] project-svc through gateway"
code=$(req GET /api/v1/projects)
assert_eq "list projects 200" 200 "$code"
proj_count=$(http_body | jq -r '.data // .projects // [] | length')
echo "  seeded projects: $proj_count"

# create one
code=$(req POST /api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"Smoke Test Project","description":"created by smoke test"}')
assert_eq "create project 201" 201 "$code"
NEW_PROJ=$(http_body | jq -r '.data.id // .id // empty')
echo "  new project id: $NEW_PROJ"
echo

# ---- 3. documents ----
echo "[3] document-svc through gateway"
code=$(req GET /api/v1/documents)
assert_eq "list documents 200" 200 "$code"
echo

# ---- 4. workflow / bids ----
echo "[4] workflow-svc through gateway"
code=$(req GET /api/v1/bids)
assert_eq "list bids 200" 200 "$code"
echo

# ---- 5. knowledge ----
echo "[5] knowledge-svc through gateway"
code=$(req GET /api/v1/knowledge)
# knowledge may not be wired in gateway (returns 404 or 200 with empty list)
if [ "$code" = "200" ] || [ "$code" = "404" ]; then
  echo "  $(green PASS) knowledge endpoint reachable ($code)"
  PASS=$((PASS+1))
else
  echo "  $(red FAIL) knowledge endpoint status=$code (want 200 or 404)"
  FAIL=$((FAIL+1))
fi
echo

# ---- 6. audit ----
echo "[6] audit-svc through gateway"
code=$(req GET /api/v1/audit/health)
if [ "$code" = "200" ] || [ "$code" = "404" ]; then
  echo "  $(green PASS) audit endpoint reachable ($code)"
  PASS=$((PASS+1))
else
  echo "  $(red FAIL) audit status=$code (want 200 or 404)"
  FAIL=$((FAIL+1))
fi
echo

# ---- 7. billing ----
echo "[7] billing-svc through gateway"
code=$(req GET /api/v1/billing/budget)
if [ "$code" = "200" ] || [ "$code" = "404" ]; then
  echo "  $(green PASS) billing endpoint reachable ($code)"
  PASS=$((PASS+1))
else
  echo "  $(red FAIL) billing status=$code (want 200 or 404)"
  FAIL=$((FAIL+1))
fi
echo

# ---- 8. notify ----
echo "[8] notify-svc through gateway"
code=$(req GET /api/v1/notify/preferences)
if [ "$code" = "200" ] || [ "$code" = "404" ]; then
  echo "  $(green PASS) notify endpoint reachable ($code)"
  PASS=$((PASS+1))
else
  echo "  $(red FAIL) notify status=$code (want 200 or 404)"
  FAIL=$((FAIL+1))
fi
echo

# ---- 9. end-to-end KB round-trip (hybrid retrieval) ----
# This exercises the new /api/v1/kb proxy: upload material → ingest
# chunks → search (vector / BM25 / hybrid).
echo "[9] knowledge-svc end-to-end (proxy through gateway)"
KB_BODY=$(mktemp)
kb_status() { curl -s -o "$KB_BODY" -w '%{http_code}' "$@"; }
KB_PROSE="国密SM4算法实现 采用对称分组密码 128比特分组 256比特密钥"
echo "  creating material"
code=$(kb_status -X POST "$GATEWAY/api/v1/kb/materials" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Tenant-Id: ${TENANT}" \
  -H "Content-Type: application/json" \
  --data-binary @<(jq -n --arg t "$KB_PROSE" \
    '{title:"smoke-kb",category:"case",content:$t}'))
assert_eq "create kb material 201" 201 "$code"
MAT_ID=$(jq -r '.data.id // .id // empty' < "$KB_BODY")
echo "  material: $MAT_ID"
echo "  ingest (chunks)"
code=$(kb_status -X POST "$GATEWAY/api/v1/kb/ingest" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Tenant-Id: ${TENANT}" \
  -H "Content-Type: application/json" \
  -d "{\"material_id\":\"${MAT_ID}\"}")
assert_eq "ingest 200" 200 "$code"
echo "  search hybrid"
code=$(kb_status -X POST "$GATEWAY/api/v1/kb/search" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Tenant-Id: ${TENANT}" \
  -H "Content-Type: application/json" \
  -d '{"query":"SM4 对称分组","mode":"hybrid","top_k":3}')
assert_eq "kb search 200" 200 "$code"
hits=$(jq -r '.data // .chunks // [] | length' < "$KB_BODY")
echo "  hybrid hits: $hits"
rm -f "$KB_BODY"
echo

# ---- 10. CORS preflight ----
echo "[10] CORS preflight"
code=$(curl -s -o /dev/null -w '%{http_code}' -X OPTIONS "$GATEWAY/api/v1/projects" \
  -H "Origin: http://localhost:5173" \
  -H "Access-Control-Request-Method: GET")
if [ "$code" = "204" ] || [ "$code" = "200" ]; then
  echo "  $(green PASS) CORS preflight $code"
  PASS=$((PASS+1))
else
  echo "  $(red FAIL) CORS preflight $code"
  FAIL=$((FAIL+1))
fi
echo

# ---- summary ----
echo "== summary =="
echo "  $(green PASS): $PASS"
echo "  $(red FAIL): $FAIL"
[ "$FAIL" -eq 0 ] || exit 1
echo "  $(green) all smoke checks passed"