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
  # GAGGLE_DEPLOY_KEY arrives base64-encoded (single line) and is decoded here.
  printf '%s\n' "$GAGGLE_DEPLOY_KEY" | base64 -d > "$SSH_DIR/id_deploy"
  # base64 -d strips trailing whitespace; OpenSSH rejects a private key whose
  # END line has no trailing newline ("invalid format"). Restore it if missing.
  [ -z "$(tail -c 1 "$SSH_DIR/id_deploy" | tr -d '\n')" ] || printf '\n' >> "$SSH_DIR/id_deploy"
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
WEB_HTTPS_PORT=443
HTTPS_DOMAIN=${GAGGLE_HTTPS_DOMAIN:-}
EOF
}

deploy() {
  cd "$DEPLOY_DIR"
  # --no-cache: web/public assets (favicon.ico, gaggle-goose.png) keep FIXED
  # filenames, so a stale cached dist layer can leave them corrupt/unreadable
  # while the content-hashed /assets/* still refresh — nginx then 403s the
  # favicon + sidebar logo with no code change. Rebuilding uncached guarantees
  # the baked artifacts match this checkout every deploy (costs ~1-2 extra min).
  docker compose -f compose.yaml -f compose.prod.yaml build --no-cache
  # --wait blocks until every service reports healthy (or the 5min timeout
  # hits), so the health check below never races a just-booted container.
  # Both api and web carry healthchecks; services without one (certbot) are
  # considered ready once running.
  docker compose -f compose.yaml -f compose.prod.yaml up -d --wait --wait-timeout 300
}

health_check() {
  docker compose -f compose.yaml -f compose.prod.yaml ps --status running api web > /dev/null
  # Final gate over the real production path: web is already healthy (up
  # --wait above), so this HTTPS probe must succeed — a failure here means a
  # genuinely broken TLS/SPA/proxy, not a container that just hadn't booted.
  # TLS is self-signed until a real cert is issued, so curl skips
  # verification for the HTTPS health check.
  curl -kfsS https://localhost/swagger/doc.json > /dev/null
  # Fixed-name public assets must actually be served (not 403 from a stale
  # layer). The SPA fallback would return 200 for a MISSING file, so assert the
  # real HTTP status via -f (fails on >= 400).
  for p in /favicon.ico /gaggle-goose.png; do
    if ! curl -kfsS -o /dev/null "https://localhost$p"; then
      echo ">> static asset $p is not being served (nginx 403/404?)" >&2
      exit 1
    fi
  done
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