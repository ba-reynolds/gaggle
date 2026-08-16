#!/usr/bin/env bash
# Starts/stops local Postgres + Redis for development and testing without Docker.
# Usage: scripts/dev-services.sh {start|stop|status}
set -euo pipefail

PGDATA="${PGDATA:-/tmp/gophersocial/pgdata}"
PGLOG="${PGLOG:-/tmp/gophersocial/pg.log}"
REDISDIR="${REDISDIR:-/tmp/gophersocial/redis}"
REDIS_PID="${REDISDIR}/redis.pid"

DB_PORT="${DB_PORT:-6969}"
DB_USER="${DB_USER:-white}"

PG_OPTS="-p ${DB_PORT} -c listen_addresses=localhost -c unix_socket_directories=/tmp"

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "error: '$1' not found. Run inside the nix dev shell: nix develop"; exit 1; }
}

start() {
  require initdb && require pg_ctl && require redis-server
  mkdir -p "${REDISDIR}"

  if [ ! -f "${PGDATA}/PG_VERSION" ]; then
    echo ">> initializing Postgres data dir at ${PGDATA}"
    initdb -D "${PGDATA}" -U "${DB_USER}" --auth=trust >/dev/null
  fi

  if ! pg_ctl -D "${PGDATA}" status >/dev/null 2>&1; then
    echo ">> starting Postgres on port ${DB_PORT}"
    pg_ctl -D "${PGDATA}" -l "${PGLOG}" -o "${PG_OPTS}" start >/dev/null
    sleep 2
  fi

  if ! redis-cli -p 6379 ping >/dev/null 2>&1; then
    echo ">> starting Redis on port 6379"
    redis-server --port 6379 --daemonize yes --dir "${REDISDIR}" --pidfile "${REDIS_PID}" >/dev/null
    sleep 1
  fi

  echo ">> services ready: postgres :${DB_PORT} (user=${DB_USER}) | redis :6379"
}

stop() {
  require pg_ctl
  if pg_ctl -D "${PGDATA}" status >/dev/null 2>&1; then
    echo ">> stopping Postgres"
    pg_ctl -D "${PGDATA}" stop >/dev/null
  fi
  if [ -f "${REDIS_PID}" ] && kill -0 "$(cat "${REDIS_PID}")" 2>/dev/null; then
    echo ">> stopping Redis"
    redis-cli -p 6379 shutdown nosave >/dev/null 2>&1 || true
  fi
  echo ">> services stopped"
}

status() {
  echo -n "postgres: "
  pg_ctl -D "${PGDATA}" status >/dev/null 2>&1 && echo "running on :${DB_PORT}" || echo "stopped"
  echo -n "redis:    "
  redis-cli -p 6379 ping >/dev/null 2>&1 && echo "running on :6379" || echo "stopped"
}

case "${1:-start}" in
  start) start ;;
  stop) stop ;;
  status) status ;;
  *) echo "Usage: $0 {start|stop|status}"; exit 1 ;;
esac
