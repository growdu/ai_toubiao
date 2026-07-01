#!/bin/bash
# Start all BidWriter services locally (no Docker for app services).
# Infra (postgres + minio) is assumed already up via docker-compose.infra.yml.
#
# Usage:
#   ./scripts/start-services.sh          # start everything, wait for ready
#   ./scripts/start-services.sh stop    # stop all
#   ./scripts/start-services.sh status   # show PIDs + last health state
#
# Service ports (match api-gateway's expected upstream URLs).
# api-gateway uses :7080 (not :8080) because code-server already owns :8080 on this host.
#   api-gateway    :7080   (public entry)
#   project-svc    :8081
#   document-svc   :8082
#   workflow-svc   :8083   (api-gateway expects 8083, workflow default is 9083)
#   router-svc     :8085
#   knowledge-svc  :8086
#   audit-svc      :8095
#   template-svc   :8096
#   billing-svc    :8097
#   notify-svc     :8098
set -eo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG_DIR="${ROOT}/.run/logs"
PID_DIR="${ROOT}/.run/pids"
DB_DSN="${DB_DSN:-postgres://postgres:postgres@localhost:5434/bidwriter?sslmode=disable}"
JWT_SECRET="${JWT_SECRET:-dev-secret-smoke-test-min-32-chars-long}"

mkdir -p "$LOG_DIR" "$PID_DIR"

# Common env exports every service picks up.
export DB_DSN DATABASE_DSN DATABASE_URL
DB_DSN="$DB_DSN"
DATABASE_DSN="$DB_DSN"
DATABASE_URL="$DB_DSN"
export JWT_SECRET

# Per-service URL exports (api-gateway / workflow need them)
export PROJECT_SVC_URL="http://localhost:8081"
export DOCUMENT_SVC_URL="http://localhost:8082"
export WORKFLOW_SVC_URL="http://localhost:8083"
export ROUTER_URL="http://localhost:8083"
export KNOWLEDGE_URL="http://localhost:8086"
export DOCUMENT_URL="http://localhost:8082"
export AUDIT_URL="http://localhost:8095"

# Storage / mock-provider flags
export STORAGE_KIND="minio"
export MINIO_ENDPOINT="localhost:9000"
export MINIO_BUCKET="bidwriter"
export MINIO_ACCESS_KEY="minioadmin"
export MINIO_SECRET_KEY="minioadmin"
export MINIO_USE_SSL="false"
export TEMPLATE_STORAGE_KIND="minio"
export TEMPLATE_MINIO_BUCKET="templates"
export ALLOW_MOCK_PROVIDER="true"
export AUTH_REQUIRED="false"   # router-svc dev: don't enforce JWT
# (router-svc uses DATABASE_URL not DB_DSN)

start_service() {
  local svc="$1" addr="$2"
  local logfile="$LOG_DIR/${svc}.log"
  local pidfile="$PID_DIR/${svc}.pid"
  if [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    echo "  $svc already running (pid $(cat "$pidfile"))"
    return 0
  fi
  echo "  starting $svc on $addr (log: $logfile)"
  HTTP_ADDR="$addr" \
  SERVICE_NAME="$svc" \
  nohup go run "./services/$svc/cmd/$svc" >"$logfile" 2>&1 &
  echo $! > "$pidfile"
}

start_all() {
  echo "== starting services (DB=$DB_DSN) =="
  cd "$ROOT"
  start_service project-svc    :8081
  start_service document-svc   :8082
  start_service workflow-svc   :8083
  start_service router-svc     :8085
  start_service knowledge-svc  :8086
  start_service audit-svc      :8095
  start_service template-svc   :8096
  start_service billing-svc    :8097
  start_service notify-svc     :8098
  # api-gateway last (others must be up so the proxy targets resolve)
  # Use :7080 to avoid clashing with code-server on :8080.
  start_service api-gateway    :7080

  echo
  echo "== waiting for health endpoints =="
  local -A ports=(
    [api-gateway]=7080 [project-svc]=8081 [document-svc]=8082
    [workflow-svc]=8083 [router-svc]=8085 [knowledge-svc]=8086
    [audit-svc]=8095 [template-svc]=8096 [billing-svc]=8097 [notify-svc]=8098
  )
  for svc in "${!ports[@]}"; do
    local p="${ports[$svc]}"
    if wait_for "http://localhost:$p/healthz" "$svc"; then
      echo "  ✓ $svc :$p ready"
    else
      echo "  ✗ $svc :$p NOT ready (last log lines:)"
      tail -5 "$LOG_DIR/${svc}.log" 2>/dev/null | sed 's/^/      /'
    fi
  done
}

wait_for() {
  local url="$1" name="$2"
  for i in {1..40}; do  # 40 * 0.5s = 20s
    if curl -sf -o /dev/null --max-time 1 "$url"; then
      return 0
    fi
    # Special case: some services don't expose /healthz, try root.
    if [ "$name" != "api-gateway" ] && curl -sf -o /dev/null --max-time 1 "http://localhost:${url##*:}"; then
      # 200 or 404 both mean the port is up
      local code
      code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 1 "http://localhost:${url##*:}/")
      if [ "$code" != "000" ]; then return 0; fi
    fi
    sleep 0.5
  done
  return 1
}

stop_all() {
  echo "== stopping services =="
  for pidfile in "$PID_DIR"/*.pid; do
    [ -f "$pidfile" ] || continue
    local svc pid
    svc=$(basename "$pidfile" .pid)
    pid=$(cat "$pidfile")
    if kill -0 "$pid" 2>/dev/null; then
      # Also kill any go-build child processes spawned by `go run`.
      pkill -P "$pid" 2>/dev/null || true
      kill "$pid" 2>/dev/null || true
      echo "  stopped $svc (pid $pid)"
    fi
    rm -f "$pidfile"
  done
}

status_all() {
  for pidfile in "$PID_DIR"/*.pid; do
    [ -f "$pidfile" ] || continue
    local svc pid
    svc=$(basename "$pidfile" .pid)
    pid=$(cat "$pidfile")
    if kill -0 "$pid" 2>/dev/null; then
      echo "  $svc: running (pid $pid)"
    else
      echo "  $svc: dead"
    fi
  done
}

case "${1:-start}" in
  start)   start_all ;;
  stop)    stop_all ;;
  status)  status_all ;;
  restart) stop_all; sleep 1; start_all ;;
  *) echo "usage: $0 {start|stop|status|restart}"; exit 1 ;;
esac