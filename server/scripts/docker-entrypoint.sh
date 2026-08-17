#!/bin/sh
# Runs database migrations (idempotent) then starts the API server.
# The compose pipeline guarantees Postgres is healthy before this runs, but
# we retry briefly to be resilient. Disable with MIGRATE_ON_START=false.
set -e

if [ "${MIGRATE_ON_START:-true}" = "true" ]; then
  echo ">> applying migrations (MIGRATE_ON_START=true)"
  count=0
  until /app/migrate -path /app/migrations -database "$POSTGRES_URL" up; do
    count=$((count + 1))
    if [ "$count" -ge 30 ]; then
      echo ">> migrations failed after $count attempts; giving up" >&2
      exit 1
    fi
    echo ">> database not ready, retrying (${count}/30)"
    sleep 2
  done
fi

echo ">> starting API"
exec /app/api
