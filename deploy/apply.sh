#!/usr/bin/env bash
# Executed on the EC2 box by .github/workflows/deploy.yml over SSH.
# Reads runtime secrets from /tmp/gaggle-env (written by the runner) and
# deploys the target SHA. Rollback = checkout the previous SHA and re-run.
set -euo pipefail
umask 077

ENV_FILE="${GAGGLE_ENV_FILE:-/tmp/gaggle-env}"
DEPLOY_DIR=/srv/gaggle
SSH_DIR=/home/deploy/.ssh
GIT_URL=git@github.com:ba-reynolds/gaggle.git

if [[ ! -r "$ENV_FILE" ]]; then
  echo "runtime env missing at $ENV_FILE" >&2
  exit 1
fi
set -a
source "$ENV_FILE"
set +a

: "${TARGET_SHA:?TARGET_SHA required}"
: "${GAGGLE_JWT_SECRET:?GAGGLE_JWT_SECRET required}"
: "${GAGGLE_DB_PASSWORD:?GAGGLE_DB_PASSWORD required}"
: "${GAGGLE_DEPLOY_KEY:?GAGGLE_DEPLOY_KEY required}"
GAGGLE_DB_USER="${GAGGLE_DB_USER:-gaggle}"

install_git_access() {
  mkdir -p "$SSH_DIR"
  chmod 700 "$SSH_DIR"
  printf '%s\n' "$GAGGLE_DEPLOY_KEY" > "$SSH_DIR/id_deploy"
  chmod 600 "$SSH_DIR/id_deploy"
  export GIT_SSH_COMMAND="ssh -i $SSH_DIR/id_deploy -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
  ssh-keyscan -H github.com >> "$SSH_DIR/known_hosts" 2>/dev/null || true
}

checkout() {
  if [[ ! -d "$DEPLOY_DIR/.git" ]]; then
    mkdir -p "$DEPLOY_DIR"
    git clone "$GIT_URL" "$DEPLOY_DIR"
  fi
  cd "$DEPLOY_DIR"
  git remote set-url origin "$GIT_URL"
  git fetch --depth 1 origin main
  git fetch --depth 1 origin "$TARGET_SHA" 2>/dev/null || true
  git checkout --force --detach "$TARGET_SHA"
}

write_env() {
  cat > "$DEPLOY_DIR/.env" <<EOF
DB_USER=$GAGGLE_DB_USER
DB_PASSWORD=$GAGGLE_DB_PASSWORD
JWT_SECRET=$GAGGLE_JWT_SECRET
WEB_PORT=80
EOF
}

deploy() {
  cd "$DEPLOY_DIR"
  docker compose -f compose.yaml -f compose.prod.yaml build
  docker compose -f compose.yaml -f compose.prod.yaml up -d
}

health_check() {
  docker compose -f compose.yaml -f compose.prod.yaml ps --status running api web > /dev/null
  curl -fsS http://localhost/swagger/doc.json > /dev/null
}

main() {
  install_git_access
  checkout
  write_env
  deploy
  if ! health_check; then
    echo ">> health check FAILED" >&2
    docker compose -f compose.yaml -f compose.prod.yaml logs --tail=100 api || true
    exit 1
  fi
  echo ">> deployed $TARGET_SHA OK"
}

main "$@"