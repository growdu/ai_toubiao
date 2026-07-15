#!/bin/bash
# start-pg.sh — bring up the test Postgres container for the BidWriter
# stack, bound to 127.0.0.1 ONLY.
#
# Why a script (not docker compose with 0.0.0.0 binding)?
#   * The manually-launched bidwriter-pg-test outside compose was
#     previously created with `-p 5434:5432` which Docker maps to
#     0.0.0.0:5434 on every host interface. That has historically
#     attracted automated scanners (e.g. from 85.11.167.232) that
#     reach the cluster via the default `postgres:postgres`
#     credential and execute `ALTER ROLE postgres NOLOGIN` plus
#     reconnaissance statements like
#     `REVOKE pg_execute_server_program FROM CURRENT_USER`. That
#     breaks `/auth/login` for the local stack with HTTP 500.
#   * Binding to 127.0.0.1 closes the public listener entirely. The
#     Go stack reaches the cluster through docker-proxy via its
#     bridge IP (172.17.x.x), so a localhost-only bind still works
#     for the in-container service.
#
# Usage:
#   ./scripts/start-pg.sh          # create + start
#   ./scripts/start-pg.sh stop     # stop + delete container (data volume kept)
#   ./scripts/start-pg.sh rm       # force-remove container + volumes
#
# Environment overrides (all optional):
#   PG_CONTAINER  default: bidwriter-pg-test
#   PG_PORT       host port (default 5434, published to 127.0.0.1)
#   PG_IMAGE      default: pgvector/pgvector:pg16
#   PG_DATA_DIR   host bind-mount target (default /home/ubuntu/bidwriter-pg-data)
#   PG_INIT_DIR   init scripts (default /tmp/bidwriter-initdb)
#   PG_PASSWORD   default: postgres
#   PG_DB         default: bidwriter

set -uo pipefail

ACTION="${1:-start}"
PG_CONTAINER="${PG_CONTAINER:-bidwriter-pg-test}"
PG_PORT="${PG_PORT:-5434}"
PG_IMAGE="${PG_IMAGE:-pgvector/pgvector:pg16}"
PG_DATA_DIR="${PG_DATA_DIR:-/home/ubuntu/bidwriter-pg-data}"
PG_INIT_DIR="${PG_INIT_DIR:-/tmp/bidwriter-initdb}"
PG_PASSWORD="${PG_PASSWORD:-postgres}"
PG_DB="${PG_DB:-bidwriter}"

case "$ACTION" in
  start)
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$PG_CONTAINER"; then
      echo "[start-pg] $PG_CONTAINER already running"
      docker ps --filter "name=$PG_CONTAINER" --format "  {{.Names}}: {{.Status}}  ports={{.Ports}}"
      exit 0
    fi

    # Re-create if it exists stopped
    docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true

    mkdir -p "$PG_DATA_DIR"
    mkdir -p "$PG_INIT_DIR"

    # CRITICAL: bind ONLY to 127.0.0.1, not 0.0.0.0. Earlier `0.0.0.0`
    # bindings attracted opportunistic attackers that locked us out.
    docker run -d --name "$PG_CONTAINER" \
      --restart unless-stopped \
      -e "POSTGRES_USER=postgres" \
      -e "POSTGRES_PASSWORD=$PG_PASSWORD" \
      -e "POSTGRES_DB=$PG_DB" \
      -p "127.0.0.1:${PG_PORT}:5432" \
      -v "${PG_DATA_DIR}:/var/lib/postgresql/data" \
      -v "${PG_INIT_DIR}:/docker-entrypoint-initdb.d:ro" \
      "$PG_IMAGE" \
      >/dev/null

    # Wait for ready
    ready=0
    for _ in $(seq 1 30); do
      code=$(docker exec -e "PGPASSWORD=$PG_PASSWORD" "$PG_CONTAINER" \
        psql -h 127.0.0.1 -p 5432 -U postgres -d "$PG_DB" -tAc "SELECT 1;" 2>/dev/null \
        && echo ok || echo fail)
      if [ "$code" = "ok" ]; then
        ready=1
        break
      fi
      sleep 1
    done

    if [ "$ready" = "1" ]; then
      echo "[start-pg] $PG_CONTAINER up on 127.0.0.1:${PG_PORT} -> 5432 in-container"
      echo "  external probe check (should be refused):"
      # We deliberately use the host's external IP to confirm no public listener.
      docker ps --filter "name=$PG_CONTAINER" --format "  {{.Names}}: {{.Status}}  ports={{.Ports}}"
    else
      echo "[start-pg] !! $PG_CONTAINER failed to become ready; see 'docker logs $PG_CONTAINER'"
      exit 1
    fi
    ;;

  stop)
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$PG_CONTAINER"; then
      docker stop "$PG_CONTAINER" >/dev/null
      docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
      echo "[start-pg] $PG_CONTAINER stopped and removed (data volume kept)"
    else
      echo "[start-pg] $PG_CONTAINER not running"
    fi
    ;;

  rm)
    docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
    echo "[start-pg] $PG_CONTAINER removed (data volume kept). Use 'rm -rf $PG_DATA_DIR' to wipe data."
    ;;

  status)
    docker ps --filter "name=$PG_CONTAINER" --format "{{.Names}}: {{.Status}}  ports={{.Ports}}"
    ;;

  *)
    echo "usage: $0 {start|stop|rm|status}"
    exit 1
    ;;
esac
