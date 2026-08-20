#!/usr/bin/env bash
# Stand up an isolated preview stack for one worktree. Each project gets its
# own db/redis/api/web containers and volumes (docker compose -p namespaces
# them automatically) on ports derived from a deterministic hash of the
# project name and PERSISTED in ~/.local/state/gaggle-proj/<project>.env so a
# re-run keeps the SAME ports (an agent reports a frontend URL; it must stay
# valid).
#
# Usage: proj-up.sh <project-name>   e.g. proj-up.sh gaggle-message-grouping
# Run via `make proj-dev` inside an agent-branch/<slug> worktree.
set -euo pipefail

PROJ="${1:?usage: proj-up.sh <project-name>}"
SLUG="${PROJ#gaggle-}"

STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/gaggle-proj"
STATE_FILE="$STATE_DIR/$PROJ.env"
mkdir -p "$STATE_DIR"

port_in_use() { ss -ltn 2>/dev/null | grep -qE "[:.]$1[[:space:]]"; }

if [ -f "$STATE_FILE" ]; then
  API_PORT="$(awk -F= '/^API_PORT=/{print $2}' "$STATE_FILE")"
  WEB_PORT="$(awk -F= '/^WEB_PORT=/{print $2}' "$STATE_FILE")"
  DB_PORT="$(awk -F= '/^DB_PORT=/{print $2}' "$STATE_FILE")"
  REDIS_PORT="$(awk -F= '/^REDIS_PORT=/{print $2}' "$STATE_FILE")"
  [ -n "${API_PORT:-}" ] || API_PORT=""
  [ -n "${DB_PORT:-}" ] || DB_PORT=""
else
  H=$(( 0x$(printf '%s' "$SLUG" | sha256sum | cut -c1-4) ))
  API_PORT=$(( 2100 + H % 200 ))
  WEB_PORT=$(( 5200 + H % 200 ))
  DB_PORT=$(( 6970 + H % 80 ))
  REDIS_PORT=$(( 6380 + H % 80 ))
  while port_in_use "$API_PORT"; do API_PORT=$(( API_PORT + 1 )); done
  while port_in_use "$WEB_PORT"; do WEB_PORT=$(( WEB_PORT + 1 )); done
  while port_in_use "$DB_PORT"; do DB_PORT=$(( DB_PORT + 1 )); done
  while port_in_use "$REDIS_PORT"; do REDIS_PORT=$(( REDIS_PORT + 1 )); done
  printf 'API_PORT=%s\nWEB_PORT=%s\nDB_PORT=%s\nREDIS_PORT=%s\n' "$API_PORT" "$WEB_PORT" "$DB_PORT" "$REDIS_PORT" > "$STATE_FILE"
fi
# Backfill DB/REDIS for state files created before this change.
if [ -z "${DB_PORT:-}" ]; then
  H2=$(( 0x$(printf '%s' "$SLUG" | sha256sum | cut -c1-4) ))
  DB_PORT=$(( 6970 + H2 % 80 )); while port_in_use "$DB_PORT"; do DB_PORT=$(( DB_PORT + 1 )); done
  REDIS_PORT=$(( 6380 + H2 % 80 )); while port_in_use "$REDIS_PORT"; do REDIS_PORT=$(( REDIS_PORT + 1 )); done
  if grep -q "^DB_PORT=" "$STATE_FILE" 2>/dev/null; then
    sed -i "s/^DB_PORT=.*/DB_PORT=$DB_PORT/; s/^REDIS_PORT=.*/REDIS_PORT=$REDIS_PORT/" "$STATE_FILE"
  else
    printf 'DB_PORT=%s\nREDIS_PORT=%s\n' "$DB_PORT" "$REDIS_PORT" >> "$STATE_FILE"
  fi
fi

export API_PORT WEB_PORT DB_PORT REDIS_PORT
# Selective build: in auto mode, only rebuild images whose sources changed.
# BUILD_MODE=auto (default) | full | web-only | api-only | none (reuse images).
# Uses git diff against HEAD (uncommitted + staged) plus untracked files.
BUILD_MODE="${BUILD_MODE:-auto}"
if [ "$BUILD_MODE" = "auto" ]; then
  REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
  CHANGED="$(git -C "$REPO_ROOT" diff --name-only HEAD 2>/dev/null; git -C "$REPO_ROOT" diff --name-only --cached 2>/dev/null; git -C "$REPO_ROOT" ls-files --others --exclude-standard 2>/dev/null)"
  # Trim to avoid huge lists; empty means no git repo or no changes.
  CHANGED="$(echo "$CHANGED" | head -n 500)"
  if [ -n "$CHANGED" ]; then
    HAS_WEB=0; HAS_API=0
    if echo "$CHANGED" | grep -qE '^(web/|compose\.yaml|compose\.prod\.yaml|infra/|scripts/proj-up\.sh)'; then HAS_WEB=1; fi
    if echo "$CHANGED" | grep -qE '^(server/|compose\.yaml|compose\.prod\.yaml|infra/|scripts/proj-up\.sh)'; then HAS_API=1; fi
    # If compose/infra/scripts changed, be conservative and build both.
    if echo "$CHANGED" | grep -qE '^(compose\.yaml|compose\.prod\.yaml|infra/|scripts/)'; then BUILD_MODE="full"
    elif [ "$HAS_WEB" = 1 ] && [ "$HAS_API" = 0 ]; then BUILD_MODE="web-only"
    elif [ "$HAS_API" = 1 ] && [ "$HAS_WEB" = 0 ]; then BUILD_MODE="api-only"
    elif [ "$HAS_WEB" = 0 ] && [ "$HAS_API" = 0 ]; then BUILD_MODE="none"
    else BUILD_MODE="full"; fi
  else
    # No git diff (not a git repo or no changes) — build both.
    BUILD_MODE="full"
  fi
fi
START_TS=$(date +%s)
echo ">> starting isolated preview $PROJ (api :$API_PORT, web :$WEB_PORT, db :$DB_PORT, redis :$REDIS_PORT) [build=$BUILD_MODE]"
case "$BUILD_MODE" in
  web-only) docker compose -p "$PROJ" build web ;;
  api-only) docker compose -p "$PROJ" build api ;;
  none) ;;
  *) docker compose -p "$PROJ" build ;;
esac
docker compose -p "$PROJ" up -d --wait
END_TS=$(date +%s); echo ">> preview ready in $((END_TS - START_TS))s [build=$BUILD_MODE]"

echo
echo "  Frontend: http://localhost:$WEB_PORT"
echo "  API:      http://localhost:$API_PORT   (swagger: /swagger)"
echo "  Seed:     make proj-seed      Logs: make proj-logs      Stop: make proj-stop"