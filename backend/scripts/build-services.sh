#!/bin/bash
# build-services.sh — compile all 11 Go services into static binaries.
#
# Why this script exists:
#   * The host does NOT have Go installed (we run the compiler in a
#     disposable golang:alpine container).
#   * The 10 services share a Go workspace (backend/go.work); a single
#     container can build them all in one go.work resolution cycle.
#   * Binaries land in /tmp/bidwriter-bin on the host so the
#     start-stack container can mount them read-only.
#
# Usage:
#   ./scripts/build-services.sh                 # build all
#   ./scripts/build-services.sh api-gateway     # build one
#
# Requirements:
#   * docker
#   * host.docker.internal reachable from inside the container
#     (docker run --add-host host.docker.internal:host-gateway handles
#     this on Linux; on macOS/Windows it's automatic)
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="${OUT_DIR:-/tmp/bidwriter-bin}"
BUILDER_IMAGE="${BUILDER_IMAGE:-golang:1.25-alpine}"
GO_WORK="$ROOT/go.work"

if ! command -v docker >/dev/null 2>&1; then
  echo "fatal: docker not in PATH" >&2
  exit 1
fi
if [ ! -f "$GO_WORK" ]; then
  echo "fatal: go.work not found at $GO_WORK" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

# Service names from go.work's use() list. Order doesn't matter for
# compilation; we just iterate.
SERVICES=(
  api-gateway
  project-svc
  document-svc
  workflow-svc
  router-svc
  knowledge-svc
  audit-svc
  template-svc
  billing-svc
  notify-svc
  docgen-svc
)

# If the user passed a service name on the CLI, narrow the list.
if [ "$#" -gt 0 ]; then
  SERVICES=("$@")
fi

# Use a long-running builder container so the module cache survives
# across invocations. The container just sleeps; we exec builds into it.
BUILDER_NAME="bidwriter-go-builder"

if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$BUILDER_NAME"; then
  echo "== launching builder container ($BUILDER_IMAGE) =="
  docker rm -f "$BUILDER_NAME" >/dev/null 2>&1 || true
  docker run -d \
    --name "$BUILDER_NAME" \
    -v "$ROOT":/src \
    -v "$OUT_DIR":/out \
    -w /src \
    "$BUILDER_IMAGE" \
    sh -c "while true; do sleep 3600; done" >/dev/null
fi

echo "== building ${SERVICES[*]} =="
fail=0
# Map service name to source directory when they differ (e.g. docgen-svc -> doc-gen)
declare -A SRC_DIR=(
  [docgen-svc]=doc-gen
)
for svc in "${SERVICES[@]}"; do
  src_dir="${SRC_DIR[$svc]:-$svc}"
  echo "--- $svc ---"
  if ! docker exec "$BUILDER_NAME" \
      sh -c "cd /src/services/$src_dir && CGO_ENABLED=0 go build -o /out/$svc ./cmd/$svc" 2>&1 | tail -3; then
    echo "  FAIL: $svc"
    fail=$((fail + 1))
    continue
  fi
  if docker exec "$BUILDER_NAME" test -x "/out/$svc"; then
    size=$(stat -c%s "$OUT_DIR/$svc" 2>/dev/null || echo "?")
    echo "  OK: /out/$svc ($size bytes)"
  else
    echo "  FAIL: binary not produced"
    fail=$((fail + 1))
  fi
done

echo ""
if [ "$fail" -eq 0 ]; then
  echo "== build complete: ${#SERVICES[@]} binaries in $OUT_DIR =="
else
  echo "== build failed: $fail of ${#SERVICES[@]} services =="
  exit 1
fi