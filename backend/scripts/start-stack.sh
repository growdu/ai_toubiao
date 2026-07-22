#!/bin/bash
# start-stack.sh — bring up the full bidwriter backend stack (11 Go
# services) in one container. Requires PG/MinIO/Redis already running
# on the host (see backend/docker-compose.yml).
#
# Architecture:
#   * One alpine container (bidwriter-stack) runs as PID 1 of the
#     supervisor script (stack-entrypoint.sh).
#   * The supervisor launches 11 Go binaries with setsid so each
#     survives the parent's exit.
#   * Services listen on localhost (inside the container) on a fixed
#     port plan; api-gateway is the only externally-reachable one
#     (via the docker port mapping below).
#
# First-time setup:
#   1. ./scripts/build-services.sh          # compile 10 binaries to /tmp/bidwriter-bin
#   2. ./scripts/start-stack.sh             # run this script
#   3. docker logs -f bidwriter-stack       # tail per-service logs
#
# Tear down:
#   ./scripts/start-stack.sh stop
#
# Why not 10 separate docker containers?
#   * api-gateway upstream URLs need to resolve to host:port. With
#     10 containers, we'd need either a docker network with DNS or
#     baked-in IP aliases. Single container = localhost everywhere.
#   * One stop signal = clean shutdown of all 10. No race on shared
#     MinIO/PG connection pools.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${BIN_DIR:-/tmp/bidwriter-bin}"
LOG_DIR="${LOG_DIR:-/tmp/bidwriter-logs}"
PID_DIR="${PID_DIR:-/tmp/bidwriter-pids}"
EP_DIR="$ROOT/scripts"          # stack-entrypoint.sh lives here
STACK_NAME="bidwriter-stack"
NETWORK_NAME="bidwriter-net"

mkdir -p "$LOG_DIR" "$PID_DIR"

# ---- infra endpoints (host-side, forwarded into container) ----
PG_DSN="${PG_DSN:-postgres://postgres:postgres@bidwriter-pg-test:5432/bidwriter?sslmode=disable}"
REDIS_URL="${REDIS_URL:-redis://host.docker.internal:6390/0}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-host.docker.internal:9100}"

# ---- env that every service reads ----
COMMON_ENV=(
  -e "DB_DSN=$PG_DSN"
  -e "DATABASE_DSN=$PG_DSN"
  -e "DATABASE_URL=$PG_DSN"
  -e "REDIS_URL=$REDIS_URL"
  -e "ASYNQ_REDIS_URL=$REDIS_URL"
  -e "JWT_SECRET=${JWT_SECRET:-dev-only-jwt-secret-bidwriter-stack-please-rotate-in-prod}"
  -e "JWT_TTL=${JWT_TTL:-24h}"
  -e "STORAGE_KIND=minio"
  -e "MINIO_ENDPOINT=$MINIO_ENDPOINT"
  -e "MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY:-minioadmin}"
  -e "MINIO_SECRET_KEY=${MINIO_SECRET_KEY:-minioadmin}"
  -e "MINIO_BUCKET=${MINIO_BUCKET:-bidwriter}"
  -e "MINIO_KB_BUCKET=${MINIO_KB_BUCKET:-kb-materials}"
  -e "TEMPLATE_STORAGE_KIND=minio"
  -e "TEMPLATE_MINIO_BUCKET=${TEMPLATE_MINIO_BUCKET:-templates}"
  -e "MINIO_USE_SSL=false"
  -e "MINIO_REGION=us-east-1"
  -e "ALLOW_MOCK_PROVIDER=true"
  -e "AUTH_REQUIRED=false"
  -e "LOG_LEVEL=${LOG_LEVEL:-info}"
  -e "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-${ANTHROPIC_AUTH_TOKEN}}"
  -e "ANTHROPIC_BASE_URL=${ANTHROPIC_BASE_URL:-https://api.minimaxi.com/anthropic}"
  -e "ROUTES_CONFIG_PATH=${ROUTES_CONFIG_PATH:-/src/services/router-svc/configs/routes.yaml}"
)

# ---- per-service port plan (must stay in sync with stack-entrypoint.sh) ----
declare -A PORTS=(
  [api-gateway]=7080 [project-svc]=7081 [document-svc]=7082
  [workflow-svc]=7083 [router-svc]=7085 [knowledge-svc]=7086
  [audit-svc]=7095 [template-svc]=7096 [billing-svc]=7097 [notify-svc]=7098
  [docgen-svc]=7099
)

ensure_binaries() {
  local missing=()
  for svc in "${!PORTS[@]}"; do
    [ -x "$BIN_DIR/$svc" ] || missing+=("$svc")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    echo "== binaries missing: ${missing[*]} =="
    echo "== running build-services.sh =="
    bash "$ROOT/scripts/build-services.sh" "${missing[@]}"
  fi
}

ensure_network() {
  if ! docker network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
    docker network create "$NETWORK_NAME" >/dev/null
  fi
}

# ensure_pg_login: idempotent guard against the recurring
# "postgres role is NOLOGIN" drift that breaks /auth/login (HTTP 500).
#
# Background:
#   The host-side test PG (bidwriter-pg-test) is the canonical DB for
#   the local stack. /docker-entrypoint-initdb.d/zz_998_ensure_login.sql
#   grants LOGIN on first init, but that runs only once per fresh
#   cluster. Once the data dir is reused, an external actor (an older
#   ai_teacher's PG sharing the host, a stray migration attempt, a CLI
#   poke) can flip the role back to NOLOGIN. Every Go service then
#   fails Login() with a connection error that the gateway surfaces as
#   a generic 500.
#
# Strategy:
#   1. Probe via TCP login (the same path the Go stack uses).
#   2. If LOGIN, no-op.
#   3. Otherwise try an online ALTER ROLE (cheap, no downtime).
#   4. If online ALTER is denied (cluster itself rejects logins),
#      fall back to a PG 16 single-user sidecar against the same data
#      dir, run ALTER ROLE, restart the container.
#
# Without this preflight, a single drift incident makes every
# subsequent start of the Go stack fail with 500 until an operator
# hand-fixes the role.
ensure_pg_login() {
  local pg_container="${PG_CONTAINER:-bidwriter-pg-test}"
  local pg_user="${PG_ADMIN_USER:-postgres}"
  local pg_pass="${PG_ADMIN_PASSWORD:-postgres}"
  local pg_db="${PG_DB:-bidwriter}"
  local pg_port="${PG_PORT:-5434}"

  if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$pg_container"; then
    # PG not running; not our problem. Caller will surface a clear
    # connection error if infra is missing.
    return 0
  fi

  # Probe via TCP. If the role is fine, the password works.
  if docker exec -e PGPASSWORD="$pg_pass" "$pg_container" \
      psql -h 127.0.0.1 -p 5432 -U "$pg_user" -d "$pg_db" \
        -tAc "SELECT rolcanlogin FROM pg_roles WHERE rolname='$pg_user';" 2>/dev/null \
      | grep -qx 't'; then
    return 0
  fi

  echo "  ! postgres role NOLOGIN in $pg_container; attempting online ALTER"
  if docker exec -e PGPASSWORD="$pg_pass" "$pg_container" \
      psql -h 127.0.0.1 -p 5432 -U "$pg_user" -d "$pg_db" \
        -tAc "ALTER ROLE $pg_user WITH LOGIN SUPERUSER PASSWORD '$pg_pass';" 2>/dev/null \
      | grep -qE 'ALTER ROLE'; then
    echo "  + repaired PG role $pg_user (online ALTER)"
    return 0
  fi

  echo "  ! online ALTER refused; entering single-user recovery"
  local data_dir
  data_dir=$(docker inspect "$pg_container" --format '{{range .Mounts}}{{if eq .Destination "/var/lib/postgresql/data"}}{{.Source}}{{end}}{{end}}')
  if [ -z "$data_dir" ]; then
    echo "  ! could not locate PG data dir for $pg_container"
    return 1
  fi

  docker stop "$pg_container" >/dev/null 2>&1 || true
  docker rm -f "$pg_container" >/dev/null 2>&1 || true

  local pg_image="${PG_IMAGE:-postgres:16}"
  if ! docker image inspect "$pg_image" >/dev/null 2>&1; then
    docker pull "$pg_image" >/dev/null 2>&1 || true
  fi

  docker run --rm -u root \
    -v "$data_dir":/var/lib/postgresql/data \
    --entrypoint /bin/bash "$pg_image" -lc "
      chown -R postgres:postgres /var/lib/postgresql/data || true
      rm -f /var/lib/postgresql/data/postmaster.pid
      su postgres -s /bin/bash -c '/usr/lib/postgresql/*/bin/postgres --single -D /var/lib/postgresql/data' <<SQL
ALTER ROLE $pg_user WITH LOGIN SUPERUSER PASSWORD '$pg_pass';
SQL
    " >/dev/null 2>&1

  docker start "$pg_container" >/dev/null 2>&1 \
    || (docker run -d --name "$pg_container" \
        -e POSTGRES_PASSWORD="$pg_pass" \
        -e POSTGRES_DB="$pg_db" \
        -p "${pg_port}:5432" \
        -v "$data_dir":/var/lib/postgresql/data \
        "$pg_image" >/dev/null)

  for _ in $(seq 1 30); do
    if docker exec -e PGPASSWORD="$pg_pass" "$pg_container" \
        psql -h 127.0.0.1 -p 5432 -U "$pg_user" -d "$pg_db" \
          -tAc "SELECT 1;" >/dev/null 2>&1; then
      echo "  + recovered PG role $pg_user via single-user ALTER"
      return 0
    fi
    sleep 1
  done

  echo "  ! PG recovery failed; manual intervention required"
  return 1
}

start_stack() {
  ensure_binaries
  ensure_network
  ensure_pg_login

  # If a previous run left the container, nuke it so we get a fresh
  # process tree (otherwise the pidfiles from inside the old container
  # would block restarts).
  docker rm -f "$STACK_NAME" >/dev/null 2>&1 || true

  docker run -d --name "$STACK_NAME" \
    --network "$NETWORK_NAME" \
    --add-host host.docker.internal:host-gateway \
    -p "${PORTS[api-gateway]}:${PORTS[api-gateway]}" \
    -v "$ROOT":/src \
    -v "$BIN_DIR":/bins:ro \
    -v "$LOG_DIR":/logs \
    -v "$PID_DIR":/pids \
    -v "$EP_DIR":/ep:ro \
    "${COMMON_ENV[@]}" \
    alpine:3.20 \
    sh -c "apk add --no-cache ca-certificates tzdata >/dev/null 2>&1 && /ep/stack-entrypoint.sh" \
    >/dev/null

  sleep 4
  echo "== container status =="
  docker ps | grep "$STACK_NAME" || { echo "container failed to start"; exit 1; }

  echo ""
  echo "== waiting for /healthz =="
  ready=0
  for svc in "${!PORTS[@]}"; do
    port="${PORTS[$svc]}"
    ok=0
    for i in $(seq 1 20); do
      # /healthz returns 200 on most services; 404 on those that
      # don't expose it. Either way the TCP port is bound, which is
      # what we actually want to verify here.
      code=$(docker exec "$STACK_NAME" \
        wget -q -O /dev/null --timeout=1 "http://127.0.0.1:${port}/healthz" 2>/dev/null \
        && echo 200 || echo 000)
      [ "$code" = "200" ] && ok=1 && break
      code=$(docker exec "$STACK_NAME" \
        wget -q -O /dev/null --timeout=1 "http://127.0.0.1:${port}/" 2>/dev/null \
        && echo 200 || echo 000)
      [ "$code" != "000" ] && ok=1 && break
      sleep 0.5
    done
    if [ "$ok" = "1" ]; then
      echo "  ✓ $svc :$port"
      ready=$((ready + 1))
    else
      echo "  ✗ $svc :$port"
      docker exec "$STACK_NAME" tail -5 "/logs/${svc}.log" 2>/dev/null | sed 's/^/    /'
    fi
  done

  echo ""
  echo "== summary: $ready / ${#PORTS[@]} ready =="
  echo "  api-gateway: http://127.0.0.1:${PORTS[api-gateway]}"
  echo "  per-service logs: $LOG_DIR/*.log"
}

stop_stack() {
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$STACK_NAME"; then
    echo "== stopping $STACK_NAME =="
    docker rm -f "$STACK_NAME" >/dev/null 2>&1
    # Clean up pidfiles that pointed into the now-deleted container.
    rm -f "$PID_DIR"/*.pid
    echo "  done"
  else
    echo "  $STACK_NAME not running"
  fi
}

status_stack() {
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$STACK_NAME"; then
    echo "  $STACK_NAME: running"
    echo "  ports in container:"
    docker exec "$STACK_NAME" sh -c "apk add --no-cache iproute2 >/dev/null 2>&1 && ss -tlnp 2>/dev/null | grep -E '708[0-9]|709[0-9]'" 2>/dev/null \
      | sed 's/^/    /'
  else
    echo "  $STACK_NAME: not running"
  fi
}

case "${1:-start}" in
  start)   start_stack ;;
  stop)    stop_stack ;;
  status)  status_stack ;;
  restart) stop_stack; sleep 1; start_stack ;;
  *) echo "usage: $0 {start|stop|status|restart}"; exit 1 ;;
esac