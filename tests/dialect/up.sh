#!/usr/bin/env bash
# Stands up the dialect-corpus database and runs the executor end to end:
#
#   1. starts (or reuses) the gj-test-pg Postgres container on port 5433
#   2. [default] recreates the gjtest database from scratch
#   3. applies the Atlas schema from tests/dialect/schema/backend/*.hcl
#   4. seeds sample data (tests/dialect/schema/seed.sql)
#   5. runs every .gql example in tests/dialect/ through the executor
#
# Usage:
#   tests/dialect/up.sh              # fresh DB + schema + seed + run
#   tests/dialect/up.sh --no-fresh   # skip the drop/recreate, reseed on top
#
# Requirements: docker, go. Everything else is pulled automatically.

set -euo pipefail

CONTAINER="${PG_CONTAINER:-gj-test-pg}"
PORT="${PG_PORT:-5433}"
DSN="postgres://postgres:postgres@localhost:${PORT}/gjtest?sslmode=disable"

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${DIR}/../.." && pwd)"

FRESH=1
[ "${1:-}" = "--no-fresh" ] && FRESH=0

log() { printf '\n==> %s\n' "$*"; }

# 1. container
if docker inspect "$CONTAINER" >/dev/null 2>&1; then
  log "starting existing container ${CONTAINER}"
  docker start "$CONTAINER" >/dev/null
else
  log "creating container ${CONTAINER} (postgres:17-alpine, port ${PORT})"
  docker run -d --name "$CONTAINER" \
    -e POSTGRES_PASSWORD=postgres \
    -e POSTGRES_DB=gjtest \
    -p "${PORT}:5432" \
    postgres:17-alpine >/dev/null
fi

log "waiting for postgres"
for i in $(seq 1 30); do
  if docker exec "$CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  [ "$i" = 30 ] && { echo "postgres did not come up"; exit 1; }
  sleep 1
done

# 2. fresh database
if [ "$FRESH" = 1 ]; then
  log "recreating database gjtest (drop + create)"
  docker exec "$CONTAINER" psql -U postgres -d postgres -q <<'SQL'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname IN ('gjtest', 'atlas_dev');
DROP DATABASE IF EXISTS gjtest;
CREATE DATABASE gjtest;
DROP DATABASE IF EXISTS atlas_dev;
CREATE DATABASE atlas_dev;
SQL
fi

# 3. schema from the HCL files shipped in this folder
log "applying schema from ${DIR}/schema (arigaio/atlas)"
docker run --rm --network host \
  -v "${DIR}/schema:/schema" -w /schema \
  arigaio/atlas:latest \
  schema apply --env local \
  --url "postgres://postgres:postgres@localhost:${PORT}/gjtest?sslmode=disable" \
  --dev-url "postgres://postgres:postgres@localhost:${PORT}/atlas_dev?sslmode=disable" \
  --auto-approve

# 4. seed
log "seeding sample data"
if [ "$FRESH" = 1 ]; then
  docker exec -i "$CONTAINER" psql -U postgres -d gjtest -q < "${DIR}/schema/seed.sql"
else
  # tolerate re-runs on a seeded database
  sed 's/^INSERT INTO/INSERT INTO/' "${DIR}/schema/seed.sql" \
    | docker exec -i "$CONTAINER" psql -U postgres -d gjtest -q \
    || echo "seed partially failed (probably already seeded); continuing"
fi

# 5. run the corpus
log "running the dialect corpus"
cd "${ROOT}"
GJ_TEST_PG="$DSN" go run ./tests/dialect/executor -dir tests/dialect
