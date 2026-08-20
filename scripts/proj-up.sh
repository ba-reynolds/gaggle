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
else
  H=$(( 0x$(printf '%s' "$SLUG" | sha256sum | cut -c1-4) ))
  API_PORT=$(( 2100 + H % 200 ))
  WEB_PORT=$(( 5200 + H % 200 ))
  while port_in_use "$API_PORT"; do API_PORT=$(( API_PORT + 1 )); done
  while port_in_use "$WEB_PORT"; do WEB_PORT=$(( WEB_PORT + 1 )); done
  printf 'API_PORT=%s\nWEB_PORT=%s\n' "$API_PORT" "$WEB_PORT" > "$STATE_FILE"
fi

export API_PORT WEB_PORT
echo ">> starting isolated preview $PROJ (api :$API_PORT, web :$WEB_PORT)"
docker compose -p "$PROJ" up --build -d --wait

echo
echo "  Frontend: http://localhost:$WEB_PORT"
echo "  API:      http://localhost:$API_PORT   (swagger: /swagger)"
echo "  Seed:     make proj-seed      Logs: make proj-logs      Stop: make proj-stop"