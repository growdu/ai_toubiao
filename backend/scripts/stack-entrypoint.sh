#!/bin/sh
# stack-entrypoint.sh — supervisor inside the bidwriter-stack container.
#
# This script runs as PID 1 of the bidwriter-stack container. It brings
# up all 11 Go services in dependency order, captures their stdout/stderr
# to per-service log files, and traps SIGTERM so docker stop can shut
# the whole stack down cleanly.
#
# Why a supervisor script (not docker compose with 10 services)?
#   * Single container = shared localhost, no DNS gymnastics for
#     inter-service URLs.
#   * One stop signal kills everything; no orphans.
#   * All 10 services share PG/MinIO/Redis env that was set when the
#     container was launched.
#
# Per-service env injected here (not on docker run) because it depends
# on the port plan, which is internal to this stack:
#   HTTP_ADDR / SERVICE_NAME / PORT (router-svc uses PORT)
# + the upstream URLs api-gateway needs to proxy to the others.
set -u

LOG_DIR="/logs"
PID_DIR="/pids"
mkdir -p "$LOG_DIR" "$PID_DIR"

# Port plan. Must stay in sync with start-stack.sh.
# (api-gateway is :7080 instead of the default :8080 to avoid clashing
#  with code-server / ai-teacher / other localhost consumers.)
PORTS="api-gateway:7080 project-svc:7081 document-svc:7082 workflow-svc:7083 router-svc:7085 knowledge-svc:7086 audit-svc:7095 template-svc:7096 billing-svc:7097 notify-svc:7098 docgen-svc:7099"

start_one() {
  svc="$1"; port="$2"; bin="/bins/$svc"
  logfile="$LOG_DIR/${svc}.log"
  pidfile="$PID_DIR/${svc}.pid"
  [ -x "$bin" ] || { echo "[entrypoint] ✗ $svc binary missing"; return 1; }
  if [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    return 0
  fi
  # setsid + nohup + closed stdin so the child detaches from this
  # script's controlling terminal and survives our exit.
  # HTTP_ADDR / SERVICE_NAME for the 9 standard services; PORT for
  # router-svc (the only service that reads PORT instead of HTTP_ADDR).
  # Inject REDIS_ADDR for workflow-svc (it reads REDIS_ADDR, not REDIS_URL).
  # REDIS_URL is set on docker run from the host; we derive REDIS_ADDR from it.
  # Falls back to host.docker.internal:6390 (the standard test port).
  redis_env=""
  if [ "$svc" = "workflow-svc" ]; then
    if [ -n "${REDIS_URL:-}" ]; then
      # redis://host:port/db -> host:port
      redis_host_port=$(echo "$REDIS_URL" | sed -E 's|^redis://([^/]+).*|\1|')
    else
      redis_host_port="host.docker.internal:6390"
    fi
    redis_env="REDIS_ADDR=$redis_host_port"
  fi
  setsid env \
    HTTP_ADDR=":$port" \
    SERVICE_NAME="$svc" \
    PORT="$port" \
    $redis_env \
    "$bin" >"$logfile" 2>&1 </dev/null &
  pid=$!
  echo "$pid" > "$pidfile"
  sleep 0.4
}

pids_running() {
  for f in "$PID_DIR"/*.pid; do
    [ -f "$f" ] && cat "$f"
  done
}

shutdown() {
  echo "[entrypoint] shutting down"
  for p in $(pids_running); do
    kill "$p" 2>/dev/null
  done
  sleep 1
  for p in $(pids_running); do
    kill -9 "$p" 2>/dev/null
  done
  exit 0
}
trap shutdown TERM INT

# Per-service upstream URLs. api-gateway proxies /api/v1/* to these.
# All values point to localhost inside this container.
#
# Two name conventions exist:
#   * api-gateway reads BARE names (PROJECT_SVC_URL, KNOWLEDGE_SVC_URL, …)
#   * workflow-svc / knowledge-svc read short names (DOCUMENT_URL, …)
# We export both so a single container can host both families without
# per-service env overrides.
export PROJECT_SVC_URL="http://127.0.0.1:7081"
export DOCUMENT_SVC_URL="http://127.0.0.1:7082"
export WORKFLOW_SVC_URL="http://127.0.0.1:7083"
export KNOWLEDGE_SVC_URL="http://127.0.0.1:7086"
export KNOWLEDGE_URL="http://127.0.0.1:7086"
export ROUTER_URL="http://127.0.0.1:7085"
export DOCUMENT_URL="http://127.0.0.1:7082"
export DOCGEN_URL="http://127.0.0.1:7099"
export AUDIT_SVC_URL="http://127.0.0.1:7095"
export TEMPLATE_SVC_URL="http://127.0.0.1:7096"
export BILLING_SVC_URL="http://127.0.0.1:7097"
export NOTIFY_SVC_URL="http://127.0.0.1:7098"
export DOCGEN_SVC_URL="http://127.0.0.1:7099"

# Bring services up. Order matters only for api-gateway (last) because
# it needs the upstream ports to be bound before its proxies can resolve.
for entry in project-svc:7081 document-svc:7082 workflow-svc:7083 router-svc:7085 knowledge-svc:7086 audit-svc:7095 template-svc:7096 billing-svc:7097 notify-svc:7098 docgen-svc:7099; do
  start_one "${entry%:*}" "${entry#*:}"
done
start_one api-gateway 7080

echo "[entrypoint] all started"
# Watchdog: report any child that dies so docker logs show why.
while true; do
  for f in "$PID_DIR"/*.pid; do
    [ -f "$f" ] || continue
    pid=$(cat "$f")
    svc=$(basename "$f" .pid)
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "[entrypoint] ⚠ $svc (pid=$pid) exited; see $LOG_DIR/${svc}.log"
      rm -f "$f"
    fi
  done
  sleep 5
done