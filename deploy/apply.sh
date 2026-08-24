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
  # The script runs under umask 077 (so the SSH key only when written stays
  # private), but that would leak into the checkout: git creates files with
  # `0666 & ~umask`, turning every web/public asset into a 600 file. Vite
  # copies public/ into dist/ verbatim and the image bakes those modes in →
  # nginx's worker can't read them and 403s the fixed-name assets. The key
  # already gets an explicit chmod 600, so reset to a sane umask here.
  umask 022
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
GOOGLE_CLIENT_ID=${GAGGLE_GOOGLE_CLIENT_ID:-}
GOOGLE_CLIENT_SECRET=${GAGGLE_GOOGLE_CLIENT_SECRET:-}
GOOGLE_REDIRECT_URL=${GAGGLE_GOOGLE_REDIRECT_URL:-}
GOOGLE_FRONTEND_REDIRECT_URL=${GAGGLE_FRONTEND_URL:-${HTTPS_DOMAIN:+https://$HTTPS_DOMAIN}}
FRONTEND_URL=${GAGGLE_FRONTEND_URL:-${HTTPS_DOMAIN:+https://$HTTPS_DOMAIN}}
EOF
}

# Verifies the freshly built web image carries readable fixed-name assets
# BEFORE anything is put live, so a broken image can never serve the box.
# The image tag is pinned explicitly in compose.yaml (`gaggle-web`); this
# resolves it via `config --images` rather than assuming the default
# <project>-web tag. The tag may be printed bare (`gaggle-web`) or prefixed
# with the service name (`web=gaggle-web`) depending on the compose version,
# so accept both.
verify_web_image() {
  cd "$DEPLOY_DIR"
  local img
  img=$(docker compose -f compose.yaml -f compose.prod.yaml config --images | grep -E 'gaggle-web(:latest)?$' | sed -E 's/^.*=//' | head -1)
  if [[ -z "$img" ]]; then
    echo ">> could not resolve the web image name (compose config --images)" >&2
    return 1
  fi
  echo ">> pre-flight: asserting fixed-name assets in $img"
  # Run as the nginx worker, NOT root: [ -r ] as root silently passes for 600
  # root-owned files, which is exactly how a broken image kept sailing to the
  # live box. Checking as uid 101 sees what nginx actually sees.
  docker run --rm --user nginx --entrypoint /bin/sh "$img" -c '
    for f in /usr/share/nginx/html/favicon.ico /usr/share/nginx/html/gaggle-goose.png; do
      [ -f "$f" ] || { echo "MISSING $f"; exit 1; }
      [ -s "$f" ] || { echo "EMPTY $f"; exit 1; }
      [ -r "$f" ] || { echo "UNREADABLE (worker uid) $f"; exit 1; }
    done
    ls -l /usr/share/nginx/html/favicon.ico /usr/share/nginx/html/gaggle-goose.png
  ' || {
    echo ">> web image static-asset pre-flight FAILED (see above) — aborting before publish" >&2
    return 1
  }
}

deploy() {
  cd "$DEPLOY_DIR"
  # --no-cache: web/public assets (favicon.ico, gaggle-goose.png) keep FIXED
  # filenames, so a stale cached dist layer can leave them corrupt/unreadable
  # while the content-hashed /assets/* still refresh — nginx then 403s the
  # favicon + sidebar logo with no code change. Rebuilding uncached guarantees
  # the baked artifacts match this checkout every deploy (costs ~1-2 extra min).
  docker compose -f compose.yaml -f compose.prod.yaml build --no-cache

  # A rebuilt image only helps if the container that serves it is actually
  # REPLACED: `compose up -d` does NOT recreate a container whose config hash
  # is unchanged (proven on this stack: rebuilding web to a new digest left
  # the running container serving the old image), so a --no-cache rebuild of
  # the same tag can leave the old (busted) web/api container in charge —
  # exactly how the live box kept 403ing favicon.ico/gaggle-goose.png across
  # redeploys. --force-recreate makes the freshly built image what goes live
  # every time. db/redis are recreated too (they're dependencies), but their
  # images never change and data lives in named volumes, so it's just a
  # gated restart.
  verify_web_image || return 1

  # --wait blocks until every service reports healthy (or the --wait-timeout
  # hits), so the health check below never races a just-booted container.
  # The web healthcheck now also probes the static assets, so a broken dist is
  # caught here (container stays unhealthy → up --wait fails) rather than at
  # the gate after the new container is already live.
  #
  # certbot MUST be recreated too: its entrypoint bakes HTTPS_DOMAIN in as
  # container env at create time, and `up` only recreates services whose
  # config hash changed when named explicitly... but scoping to specific
  # services SKIPS everything else entirely. Without this, setting/changing
  # GAGGLE_HTTPS_DOMAIN would never reach a sleeping certbot container.
  # Recreating is safe after first issuance: live/gaggle becomes a symlink,
  # so the entrypoint takes the renew-on-12h-loop path (no-op until expiry);
  # before issuance it (re)attempts certonly — which is exactly the retry
  # we want when a previous attempt failed on not-yet-propagated DNS.
  docker compose -f compose.yaml -f compose.prod.yaml up -d --wait --wait-timeout 300 --force-recreate api web certbot
}

health_check() {
  docker compose -f compose.yaml -f compose.prod.yaml ps --status running api web > /dev/null || return 1
  # Final gate over the real production path: web is already healthy (up
  # --wait above), so these probes must succeed — a failure here means a
  # genuinely broken proxy/SPA, not a container that just hadn't booted.
  # TLS is self-signed until a real cert is issued, so curl skips
  # verification for the HTTPS probes.
  curl -kfsS https://localhost/swagger/doc.json > /dev/null || return 1
  # Force-HTTPS policy: everything is served on HTTPS (curl -k tolerates the
  # self-signed fallback), and port 80 must 301-redirect. The SPA fallback
  # would return 200 for a MISSING file, so assert the real HTTP status via
  # -f (fails on >= 400 — catches the nginx 403 this box kept serving).
  for p in /favicon.ico /gaggle-goose.png; do
    if ! curl -kfsS -o /dev/null "https://localhost$p"; then
      echo ">> static asset $p is not being served over https (nginx 403/404?)" >&2
      return 1
    fi
    code=$(curl -ksS -o /dev/null -w '%{http_code}' "http://localhost$p" || true)
    if [ "$code" != "301" ]; then
      echo ">> http://localhost$p expected 301 redirect, got ${code:-none}" >&2
      return 1
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
    echo ">> container list:" >&2
    docker compose -f compose.yaml -f compose.prod.yaml ps >&2 || true
    echo ">> web container /usr/share/nginx/html listing:" >&2
    docker compose -f compose.yaml -f compose.prod.yaml exec -T web sh -c 'ls -la /usr/share/nginx/html/ | head -40' >&2 || true
    echo ">> api + web logs (tail 100):" >&2
    docker compose -f compose.yaml -f compose.prod.yaml logs --tail=100 api web >&2 || true
    exit 1
  fi
  echo ">> deployed $TARGET_SHA OK"
}

main "$@"