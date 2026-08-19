#!/usr/bin/env bash
# One-time provisioning helper for the gaggle AWS EC2 deploy: generates the
# SSH keys and runtime secrets you need, then (optionally) sets the GitHub
# Actions secrets via the `gh` CLI. Idempotent: re-running never overwrites
# existing keys/secrets. Run from the repo root.
#
#   bash scripts/provision.sh            # generate everything, prompt for gh
#   GAGGLE_KEY_DIR=/tmp/keys bash scripts/provision.sh
#
# What it does not do: terraform apply, registering the repo deploy key on
# GitHub, or creating the EC2 box. Those need your AWS credentials / GitHub
# web UI (see README "Production deployment").
set -euo pipefail

REPO="ba-reynolds/gaggle"
KEY_DIR="${GAGGLE_KEY_DIR:-$HOME/.gaggle-deploy}"
SECRETS_FILE="$KEY_DIR/secrets.env"

mkdir -p "$KEY_DIR"
chmod 700 "$KEY_DIR"

echo ">> using key directory: $KEY_DIR"

# --- 1. SSH keypairs (idempotent) -----------------------------------------
gen_key() {
  local name=$1
  if [[ -f "$KEY_DIR/$name" ]]; then
    echo ">> $name keypair already exists (reusing)"
    return
  fi
  echo ">> generating $name keypair"
  ssh-keygen -q -t ed25519 -N "" -C "$name" -f "$KEY_DIR/$name"
}

gen_key gaggle_admin
gen_key gaggle_ci
gen_key gaggle_repo_deploy

# --- 2. Runtime secrets (idempotent) --------------------------------------
random_hex() {
  # 32 bytes of hex from /dev/urandom; openssl isn't on every host, so fall
  # back to od (coreutils, always present on Linux).
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
  fi
}
if [[ -f "$SECRETS_FILE" ]]; then
  echo ">> secrets.env already exists (reusing)"
else
  echo ">> generating runtime secrets"
  umask 077
  {
    echo "JWT_SECRET=$(random_hex)"
    echo "DB_PASSWORD=$(random_hex)"
  } > "$SECRETS_FILE"
  chmod 600 "$SECRETS_FILE"
fi
# shellcheck disable=SC1090
source "$SECRETS_FILE"

echo
echo "============================================================"
echo " Copy these public keys into infra/terraform.tfvars:"
echo "============================================================"
echo "admin_public_key    = \"$(cat "$KEY_DIR/gaggle_admin.pub")\""
echo "deploy_public_key   = \"$(cat "$KEY_DIR/gaggle_ci.pub")\""
echo
echo " Register this as a GitHub repo deploy key (name: gaggle-box,"
echo " read-only) so the box can clone the repo:"
echo "------------------------------------------------------------"
cat "$KEY_DIR/gaggle_repo_deploy.pub"
echo "------------------------------------------------------------"
echo
echo " Runtime secrets generated in $SECRETS_FILE (values not printed;"
echo " GAGGLE_JWT_SECRET / GAGGLE_DB_PASSWORD / GAGGLE_DB_USER=gaggle)."
echo

# --- 3. GitHub secrets (opt-in, needs gh CLI) -----------------------------
if ! command -v gh >/dev/null 2>&1; then
  echo ">> gh CLI not found. Set the 6 GH secrets manually per the README."
  exit 0
fi

if [[ "${GAGGLE_SKIP_GH:-0}" == "1" ]]; then
  echo ">> GAGGLE_SKIP_GH=1, not touching GitHub secrets."
  exit 0
fi

if [[ -z "${DEPLOY_HOST:-}" ]]; then
  echo ">> DEPLOY_HOST is not set. Either export it (e.g. DEPLOY_HOST=1.2.3.4)"
  echo ">> or set it manually in GitHub after the box exists."
  DEPLOY_HOST="${DEPLOY_HOST:-<public-ip-after-terraform-apply>}"
fi

echo ">> setting GitHub Actions secrets on $REPO (requires gh auth, repo+workflow scopes)"
for keyvalue in \
  "DEPLOY_HOST=$DEPLOY_HOST" \
  "GAGGLE_JWT_SECRET=$JWT_SECRET" \
  "GAGGLE_DB_PASSWORD=$DB_PASSWORD" \
  "GAGGLE_DB_USER=gaggle"; do
  name="${keyvalue%%=*}"
  value="${keyvalue#*=}"
  gh secret set "$name" --repo "$REPO" --body "$value"
  echo "   set $name"
done

for keypair in "DEPLOY_SSH_KEY=$KEY_DIR/gaggle_ci" "GAGGLE_DEPLOY_KEY=$KEY_DIR/gaggle_repo_deploy"; do
  name="${keypair%%=*}"
  file="${keypair#*=}"
  gh secret set "$name" --repo "$REPO" --body "$(cat "$file")"
  echo "   set $name (from $file)"
done

read -r -p ">> Register the repo deploy key on GitHub now via gh? [y/N] " ans
if [[ "$ans" =~ ^[yY]$ ]]; then
  gh repo deploy-key add "$KEY_DIR/gaggle_repo_deploy.pub" \
    --repo "$REPO" --title "gaggle-box" --allow-write=false
  echo "   repo deploy key registered"
else
  echo ">> Skipped. Register it at https://github.com/$REPO/settings/keys"
fi

echo
echo "Done. Next: terraform apply in infra/, then trigger the Deploy workflow."