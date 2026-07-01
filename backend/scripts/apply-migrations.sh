#!/bin/bash
# Apply BidWriter migrations using psql by splitting each file at
# the "-- +goose Down" marker (so the Down sections are skipped).
# This avoids needing the goose binary — handy in CI/containers where
# `go install` is slow.
#
# Uses `docker exec` to run psql inside the bidwriter-postgres container
# unless DATABASE_URL points elsewhere (in which case local psql is used).
set -eo pipefail

DB_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5434/bidwriter?sslmode=disable}"
MIG_DIR="${MIGRATIONS_DIR:-$(dirname "$0")/../migrations}"
PG_CONTAINER="${PG_CONTAINER:-bidwriter-postgres}"

run_psql() {
  local sql="$1"
  if [[ "$DB_URL" == *5434* ]] && command -v docker >/dev/null 2>&1 \
       && docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$PG_CONTAINER"; then
    echo "$sql" | docker exec -i "$PG_CONTAINER" psql -U postgres -d bidwriter -v ON_ERROR_STOP=1 -X -q
  else
    psql "$DB_URL" -v ON_ERROR_STOP=1 -X -q <<<"$sql"
  fi
}

apply_file() {
  local f="$1"
  echo "-- $f"
  awk '/^-- \+goose Down[[:space:]]*$/{exit} {print}' "$f"
}

# Discover files: *.sql and *.up.sql, but not *.down.sql
mapfile -t FILES < <(cd "$MIG_DIR" && ls *.sql 2>/dev/null | grep -v '\.down\.sql$' | sort)
if [ ${#FILES[@]} -eq 0 ]; then
  echo "no migrations in $MIG_DIR" >&2
  exit 1
fi

echo "== applying ${#FILES[@]} migrations to $DB_URL =="
  for f in "${FILES[@]}"; do
    echo "-- $f"
    sql=$(apply_file "$MIG_DIR/$f")
    run_psql "$sql" || { echo "FAILED: $f" >&2; exit 1; }
  done
echo "== all migrations applied =="