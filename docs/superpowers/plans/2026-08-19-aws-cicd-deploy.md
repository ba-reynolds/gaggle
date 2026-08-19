# Gaggle AWS EC2 Deploy + GitHub CI/CD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Gaggle a GitHub CI/CD pipeline (lint+tests on PR, auto-deploy on push to `main`) and Terraform-provisioned hosting on a single AWS EC2 instance running the existing Docker Compose stack.

**Architecture:** GitHub Actions runs backend tests (Go 1.24 against `postgres:16-alpine` + `redis:7-alpine` service containers) and frontend lint/build (Node 24). On `main`, a deploy job SSHes into the EC2 box (`deploy` user), writes `.env` from GitHub secrets, checks out the target SHA, and runs `docker compose -f compose.yaml -f compose.prod.yaml up --build -d`. The box is created by Terraform (Amazon Linux 2023, `t3.small`, 20GiB root + 30GiB EBS data volume mounted at `/data` for Docker's data-root) with a user-data bootstrap script that installs Docker, sets up the `deploy` user, UFW, and fail2ban. `compose.prod.yaml` removes the public db/redis ports and sets `APP_ENV=production`, which turns on a fail-fast config check so a production instance can never boot with a shipped dev secret.

**Tech Stack:** GitHub Actions, Terraform (hashicorp/aws ~> 5.0), Amazon Linux 2023 + Docker Compose, Go 1.24 (chi, pgx), Node 24 (Vite).

**Spec:** `docs/superpowers/specs/2026-08-19-aws-cicd-deploy-design.md`

## Global Constraints

- Go module is `github.com/ba-reynolds/gaggle`, `go 1.24.3` (`server/go.mod`). Backend CI file must run in `server/` working directory.
- Frontend runs in `web/`; `npm ci` honors `allowScripts` in `package.json` (do NOT add `--ignore-scripts`).
- All production secrets come from GitHub Actions secrets → written to `/srv/gaggle/.env` on the box at deploy time. NEVER commit a real secret. `.env` and `*.env` are gitignored; `deploy/.env.production.template` (ends in `.template`) is NOT ignored and is tracked.
- Repo remote is `git@github.com:ba-reynolds/gaggle.git`. The swarm/deploy box needs read-only git access via a GitHub **deploy key** (public half registered on GitHub, private half stored as the `GAGGLE_DEPLOY_KEY` secret and injected at deploy time).
- GitHub SSH access to the box uses a separate keypair: public half goes into `deploy`'s `authorized_keys` via bootstrap; private half is the `DEPLOY_SSH_KEY` secret.
- `compose.yaml` uses `ports: "!reset []"` merge tag → requires Docker Compose v2.20+. Amazon Linux 2023's `docker-compose-plugin` is ≥2.24, acceptable.
- Terraform `user_data` uses `templatefile()`: bash vars in `bootstrap.sh` must be written WITHOUT braces (`$VAR`, not `${VAR}`) so Terraform doesn't try to interpolate them; only `admin_public_key` / `deploy_public_key` use `${...}`. Heredoc bodies in the template must avoid `${...}` or escape as `$${...}`.
- No Go/Node on the NixOS host. Use `docker compose --profile tools run --rm tools go ...` for Go, `nix shell nixpkgs#terraform --command terraform ...` for Terraform.
- Local test harness (`server/internal/testutil`) creates/drops `social_test` on a live Postgres bound via `TEST_DB_ADDRESS`; default `localhost:6969` (dev) or `localhost:5432` (CI service container).

---

### Task 1: Fail-fast config for production secrets

**Files:**
- Modify: `server/pkg/config/config.go`
- Test: `server/pkg/config/config_test.go` (create)

**Interfaces:**
- Consumes: existing `getEnv(key, default)` helper in the same package.
- Produces: `LoadConfig()` signature unchanged **but now returns an error when `APP_ENV=production` and `JWT_SECRET` or `DB_PASSWORD` is empty or a known dev default.** New behavior gate: env var `APP_ENV` (`development` default). Other tasks rely on this: `compose.prod.yaml` sets `APP_ENV=production` so the api container refuses to boot with a leaked default.

- [ ] **Step 1: Write the failing test**

Create `server/pkg/config/config_test.go`:

```go
package config

import "testing"

func TestLoadConfigProductionRejectsDevDefaults(t *testing.T) {
	devJWT := []string{"", "secret", "dev-secret-change-me", "change-me"}
	devDB := []string{"", "teeth", "password"}

	for _, val := range devJWT {
		t.Setenv("APP_ENV", "production")
		t.Setenv("DB_PASSWORD", "a-real-prod-db-password")
		t.Setenv("JWT_SECRET", val)
		if _, err := LoadConfig(); err == nil {
			t.Errorf("JWT_SECRET=%q in production should error, got nil", val)
		}
	}

	for _, val := range devDB {
		t.Setenv("APP_ENV", "production")
		t.Setenv("JWT_SECRET", "a-real-prod-jwt-secret")
		t.Setenv("DB_PASSWORD", val)
		if _, err := LoadConfig(); err == nil {
			t.Errorf("DB_PASSWORD=%q in production should error, got nil", val)
		}
	}
}

func TestLoadConfigProductionAcceptsRealSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "a-real-prod-jwt-secret")
	t.Setenv("DB_PASSWORD", "a-real-prod-db-password")
	if _, err := LoadConfig(); err != nil {
		t.Errorf("real prod secrets should load cleanly, got: %v", err)
	}
}

func TestLoadConfigDevelopmentAllowsDevDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "dev-secret-change-me")
	t.Setenv("DB_PASSWORD", "teeth")
	if _, err := LoadConfig(); err != nil {
		t.Errorf("APP_ENV=development with dev defaults should load, got: %v", err)
	}
}
```

Note: `godotenv.Load()` runs inside `LoadConfig` but never overrides already-set env vars, so `t.Setenv` wins. No `.env` exists under `server/`, so nothing else interferes.

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose --profile tools run --rm tools go test ./pkg/config/`
Expected: FAIL — `LoadConfig()` returns nil error for production, so the dev-default cases are not flagged.

- [ ] **Step 3: Implement the failsafe**

In `server/pkg/config/config.go`, add `isStrongSecret` and the production gate. The `appEnv` check must run after both `dbConfig` and `authConfig` are built (they are, as locals in `LoadConfig`).

```go
func LoadConfig() (AllConfigs, error) {
	// Load .env file if available (but don't fail if missing)
	err := godotenv.Load()
	if err != nil {
		zap.L().Warn("failed to load .env file", zap.Error(err))
	}

	// ... existing config building unchanged; dbConfig, authConfig defined here ...

	// Production fails fast: never boot with a shipped dev secret.
	if getEnv("APP_ENV", "development") == "production" {
		if !isStrongSecret(authConfig.JWTSecret) {
			return AllConfigs{}, fmt.Errorf("APP_ENV=production requires a real JWT_SECRET (got %q)", authConfig.JWTSecret)
		}
		if !isStrongSecret(dbConfig.DBPassword) {
			return AllConfigs{}, fmt.Errorf("APP_ENV=production requires a real DB_PASSWORD (got %q)", dbConfig.DBPassword)
		}
	}

	return AllConfigs{
		// ... existing return unchanged ...
	}, nil
}

// isStrongSecret rejects empty values and every secret default currently
// shipped anywhere in the repo (config.go, .env.example, compose.yaml).
func isStrongSecret(value string) bool {
	if value == "" {
		return false
	}
	for _, dev := range []string{"secret", "dev-secret-change-me", "change-me", "teeth", "password"} {
		if value == dev {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose --profile tools run --rm tools go test ./pkg/config/`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add server/pkg/config/config.go server/pkg/config/config_test.go
git commit -m "feat: fail fast on dev secrets when APP_ENV=production"
```

---

### Task 2: Production compose override

**Files:**
- Create: `compose.prod.yaml`

**Interfaces:**
- Consumes: `compose.yaml` at repo root (base service defs); `.env` written by `deploy/apply.sh` (Task 3) for compose variable substitution; `APP_ENV` failsafe from Task 1.
- Produces: `compose.prod.yaml` — merged by `docker compose -f compose.yaml -f compose.prod.yaml` on the box. Nobody publishes db/redis; web serves on host port 80; api gets `APP_ENV=production`.

- [ ] **Step 1: Write the override**

Create `compose.prod.yaml`:

```yaml
# Production override: used on the AWS EC2 box by deploy/apply.sh as
#   docker compose -f compose.yaml -f compose.prod.yaml up --build -d
#
# Differences from local dev:
#   - db/redis ports are REMOVED (reachable only on the compose network)
#   - web publishes host port 80 (set WEB_PORT=80 via /srv/gaggle/.env)
#   - APP_ENV=production enables the config fail-fast so a leaked dev
#     JWT_SECRET / DB_PASSWORD prevents the api from booting.
name: gaggle

services:
  db:
    ports: !reset []

  redis:
    ports: !reset []

  api:
    environment:
      APP_ENV: production
      COOKIE_SECURE: "false"
```

The `ports: !reset []` merge tag (Compose spec, supported since docker compose v2.20) discards the base file's published ports for db/redis. `COOKIE_SECURE` stays `"false"` until TLS lands (documented limitation; the design spec defers it).

- [ ] **Step 2: Verify the merge is valid and ports are gone**

Run:
```bash
docker compose -f compose.yaml -f compose.prod.yaml config --quiet && echo "compose merge OK"
docker compose -f compose.yaml -f compose.prod.yaml config | grep -cE '5432|6379|6969' || echo "no db/redis host ports published"
```
Expected: `compose merge OK`, then either `0` (from `grep -c`) or the `||` echo. Either way, no running container maps a db/redis port to the host.

- [ ] **Step 3: Commit**

```bash
git add compose.prod.yaml
git commit -m "feat: production compose override (no public db/redis, APP_ENV=production)"
```

---

### Task 3: Deploy script + production .env template

**Files:**
- Create: `deploy/apply.sh` (executable)
- Create: `deploy/.env.production.template`

**Interfaces:**
- Consumes: env vars injected over SSH by `deploy.yml` (Task 5) — written to `/tmp/gaggle-env` on the box and sourced: `GAGGLE_JWT_SECRET`, `GAGGLE_DB_USER`, `GAGGLE_DB_PASSWORD`, `GAGGLE_DEPLOY_KEY`, `TARGET_SHA`. Repo checked out at `/srv/gaggle` (owned by `deploy`, created by `bootstrap.sh`, Task 6).
- Produces: `/srv/gaggle/.env` (compose variable substitution + source for compose), a detached checkout at `TARGET_SHA`, and a live stack. Health check via `GET http://localhost/swagger/doc.json`.

- [ ] **Step 1: Write the deploy script**

Create `deploy/apply.sh`:

```bash
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
```

`git fetch --depth 1 origin <sha>` works on GitHub for any reachable commit. `env.production.template` below documents the resulting `.env` shape.

- [ ] **Step 2: Write the .env template**

Create `deploy/.env.production.template`:

```
# Resulting /srv/gaggle/.env on the EC2 box, written by deploy/apply.sh from
# GitHub Actions secrets at deploy time. Never commit real values.
DB_USER=gaggle
DB_PASSWORD=<from GAGGLE_DB_PASSWORD>
JWT_SECRET=<from GAGGLE_JWT_SECRET>
WEB_PORT=80
```

- [ ] **Step 3: Syntax-check the script**

Run: `bash -n deploy/apply.sh && chmod +x deploy/apply.sh && echo "syntax OK"`
Expected: `syntax OK`

- [ ] **Step 4: Commit**

```bash
git add deploy/apply.sh deploy/.env.production.template
git commit -m "feat: production deploy script and .env template"
```

---

### Task 4: CI workflow (lint, vet, tests)

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: nothing from earlier tasks (pure checks). Uses the repo's existing test harness (`server/internal/testutil`) with `TEST_DB_*` env; frontend build/lint targets from `package.json` and `Makefile`.
- Produces: reusable workflow that `deploy.yml` (Task 5) calls via `workflow_call`, plus standalone PR/push runs.

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]
  workflow_call:

jobs:
  backend:
    name: Backend (vet + test)
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: white
          POSTGRES_PASSWORD: teeth
          POSTGRES_DB: postgres
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U white"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 10
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 10
    defaults:
      run:
        working-directory: server
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: server/go.mod
          cache-dependency-path: server/go.sum
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./...
        env:
          TEST_DB_ADDRESS: localhost:5432
          TEST_DB_USER: white
          TEST_DB_PASSWORD: teeth

  frontend:
    name: Frontend (lint + build)
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '24'
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npm run build
```

The test harness's `filepath.Abs("../../cmd/migrate/migrations")` resolves relative to each test package's directory, so it finds migrations whether run from `server/` (CI) or the `tools` container (local) — no special CWD needed. The harness creates/drops `social_test` itself against the `postgres` admin DB.

- [ ] **Step 2: Validate YAML**

Run: `nix shell nixpkgs#yamllint --command yamllint .github/workflows/ci.yml && echo "YAML OK"`
Expected: no errors, `YAML OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: backend vet+test and frontend lint+build on PRs and main"
```

---

### Task 5: Deploy workflow

**Files:**
- Create: `.github/workflows/deploy.yml`

**Interfaces:**
- Consumes: `ci.yml` via `workflow_call` (Task 4), `deploy/apply.sh` (Task 3), and GitHub secrets. Required secrets (owner configures them, see Task 8 README): `DEPLOY_HOST` (public IP or DNS of the box), `DEPLOY_SSH_KEY` (private key for `deploy@`), `GAGGLE_JWT_SECRET`, `GAGGLE_DB_USER`, `GAGGLE_DB_PASSWORD`, `GAGGLE_DEPLOY_KEY`.
- Produces: triggers `deploy/apply.sh` on the box with `TARGET_SHA=$GITHUB_SHA`; fails the run if the health check fails.

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/deploy.yml`:

```yaml
name: Deploy

on:
  push:
    branches: [main]
  workflow_dispatch:

concurrency:
  group: production-deploy
  cancel-in-progress: false

jobs:
  ci:
    name: CI
    uses: ./.github/workflows/ci.yml

  deploy:
    name: Deploy to EC2
    needs: ci
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Ship and run deploy
        env:
          DEPLOY_HOST: ${{ secrets.DEPLOY_HOST }}
          DEPLOY_SSH_KEY: ${{ secrets.DEPLOY_SSH_KEY }}
          GAGGLE_JWT_SECRET: ${{ secrets.GAGGLE_JWT_SECRET }}
          GAGGLE_DB_USER: ${{ secrets.GAGGLE_DB_USER }}
          GAGGLE_DB_PASSWORD: ${{ secrets.GAGGLE_DB_PASSWORD }}
          GAGGLE_DEPLOY_KEY: ${{ secrets.GAGGLE_DEPLOY_KEY }}
          TARGET_SHA: ${{ github.sha }}
        run: |
          set -euo pipefail
          : "${DEPLOY_HOST:?DEPLOY_HOST secret not set}"
          : "${DEPLOY_SSH_KEY:?DEPLOY_SSH_KEY secret not set}"

          echo "$DEPLOY_SSH_KEY" > /tmp/gaggle_deploy_key
          chmod 600 /tmp/gaggle_deploy_key

          # Secrets cross the SSH channel only: write the runtime env into the
          # box (never the workflow logs), then pipe the apply script to bash -s.
          printf 'GAGGLE_JWT_SECRET=%s\nGAGGLE_DB_USER=%s\nGAGGLE_DB_PASSWORD=%s\nGAGGLE_DEPLOY_KEY=%s\nTARGET_SHA=%s\n' \
            "$GAGGLE_JWT_SECRET" "$GAGGLE_DB_USER" "$GAGGLE_DB_PASSWORD" "$GAGGLE_DEPLOY_KEY" "$TARGET_SHA" |
            ssh -i /tmp/gaggle_deploy_key -o StrictHostKeyChecking=accept-new "deploy@$DEPLOY_HOST" \
              'umask 077; cat > /tmp/gaggle-env'

          ssh -i /tmp/gaggle_deploy_key "deploy@$DEPLOY_HOST" 'bash -s' < deploy/apply.sh
```

- [ ] **Step 2: Validate YAML & reference**

Run: `nix shell nixpkgs#yamllint --command yamllint .github/workflows/deploy.yml && echo "YAML OK"`
Expected: no errors, `YAML OK`. (Full end-to-end can only be proven after the box exists — this is expected and deferred to the smoke test in Task 8.)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/deploy.yml
git commit -m "ci: auto-deploy to EC2 on push to main"
```

---

### Task 6: Bootstrapping the EC2 box

**Files:**
- Create: `infra/bootstrap.sh`

**Interfaces:**
- Consumes: two template variables injected by Terraform `templatefile()` (Task 7): `${admin_public_key}` and `${deploy_public_key}`.
- Produces: a box with Docker installed, `deploy` user + key-only SSH, UFW open on 22/80/443, fail2ban, `/data` EBS mount with Docker data-root on it, and a `/srv/gaggle` checkout owned by `deploy`. `deploy.yml` (Task 5) and `deploy/apply.sh` (Task 3) depend on these paths/users.

- [ ] **Step 1: Write the bootstrap script**

Create `infra/bootstrap.sh` (template placeholders on purpose — bash vars use NO braces so Terraform leaves them alone):

```bash
#!/usr/bin/env bash
# EC2 user-data bootstrap for the Gaggle box. Idempotent: safe to re-run when
# Terraform replaces user_data. Terraform interpolates the two ssh keys.
set -euo pipefail

ADMIN_PUBLIC_KEY='${admin_public_key}'
DEPLOY_PUBLIC_KEY='${deploy_public_key}'
DATA_DEV=/dev/nvme1n1
DATA_MOUNT=/data

echo ">> installing docker + compose plugin"
dnf install -y docker docker-compose-plugin
systemctl enable --now docker

echo ">> creating deploy user"
id deploy >/dev/null 2>&1 || useradd -m -s /bin/bash deploy
usermod -aG docker deploy
install -d -o deploy -g deploy -m 700 /home/deploy/.ssh
: > /home/deploy/.ssh/authorized_keys
echo "$ADMIN_PUBLIC_KEY" >> /home/deploy/.ssh/authorized_keys
echo "$DEPLOY_PUBLIC_KEY" >> /home/deploy/.ssh/authorized_keys
chown deploy:deploy /home/deploy/.ssh/authorized_keys
chmod 600 /home/deploy/.ssh/authorized_keys

echo ">> hardening sshd (key-only, no root)"
sed -i 's/^#\?PasswordAuthentication .*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#\?PermitRootLogin .*/PermitRootLogin no/' /etc/ssh/sshd_config
systemctl try-restart sshd

echo ">> firewall (UFW): 22, 80, 443"
dnf install -y ufw
ufw default deny incoming
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo ">> fail2ban"
dnf install -y fail2ban
systemctl enable --now fail2ban

echo ">> mounting data volume at /data"
if ! blkid "$DATA_DEV" >/dev/null 2>&1; then
  mkfs -t xfs "$DATA_DEV"
fi
mkdir -p "$DATA_MOUNT"
grep -q "$DATA_MOUNT" /etc/fstab || echo "$DATA_DEV $DATA_MOUNT xfs defaults,noatime 0 2" >> /etc/fstab
mountpoint -q "$DATA_MOUNT" || mount -a

echo ">> pointing docker data-root at /data/docker"
mkdir -p /data/docker
cat > /etc/docker/daemon.json <<'EOF'
{
  "data-root": "/data/docker"
}
EOF
systemctl restart docker

echo ">> cloning repo for the deploy user"
install -d -o deploy -g deploy /srv
if [[ ! -d /srv/gaggle/.git ]]; then
  # Best-effort: a fresh box has no deploy key yet, so the clone may not be
  # able to authenticate. deploy/apply.sh (via the injected GAGGLE_DEPLOY_KEY)
  # performs its own checkout on the first deploy if this is absent.
  sudo -u deploy git clone git@github.com:ba-reynolds/gaggle.git /srv/gaggle \
    || echo ">> clone deferred to first deploy (deploy key not installed yet)"
else
  echo ">> /srv/gaggle already present; leaving it to deploy/apply.sh"
fi

echo ">> bootstrap complete"
```

- [ ] **Step 2: Syntax-check**

Run: `bash -n infra/bootstrap.sh && echo "syntax OK"`
Expected: `syntax OK`

- [ ] **Step 3: Commit**

```bash
git add infra/bootstrap.sh
git commit -m "feat: EC2 user-data bootstrap (docker, deploy user, ufw, fail2ban, /data)"
```

---

### Task 7: Terraform provisioning

**Files:**
- Create: `infra/main.tf`, `infra/variables.tf`, `infra/outputs.tf`, `infra/terraform.tfvars.example`

**Interfaces:**
- Consumes: `infra/bootstrap.sh` via `templatefile` (Task 6); `var.admin_public_key` and `var.deploy_public_key` from the owner's `.tfvars`.
- Produces: EC2 instance + security group + EBS data volume attached at `/dev/sdf` (→ `/dev/nvme1n1`, matches `bootstrap.sh` `DATA_DEV`), outputs `public_ip` / `instance_id` / `data_volume_id`. The `public_ip` becomes the `DEPLOY_HOST` secret.

- [ ] **Step 1: Write main.tf**

Create `infra/main.tf`:

```hcl
terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

data "aws_vpc" "default" {
  default = true
}

resource "aws_key_pair" "admin" {
  key_name   = "gaggle-admin"
  public_key = var.admin_public_key
}

resource "aws_security_group" "gaggle" {
  name        = "gaggle-sg"
  description = "Gaggle web (80/443) + SSH (22) - key-only"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "gaggle" {
  ami                         = data.aws_ami.al2023.id
  instance_type               = var.instance_type
  vpc_security_group_ids      = [aws_security_group.gaggle.id]
  associate_public_ip_address = true
  key_name                    = aws_key_pair.admin.key_name

  user_data = templatefile("${path.module}/bootstrap.sh", {
    admin_public_key  = var.admin_public_key
    deploy_public_key = var.deploy_public_key
  })

  root_block_device {
    volume_type = "gp3"
    volume_size = var.root_volume_size
  }

  tags = {
    Name = "gaggle-web"
  }
}

resource "aws_ebs_volume" "data" {
  availability_zone = aws_instance.gaggle.availability_zone
  size              = var.data_volume_size
  type              = "gp3"

  tags = {
    Name = "gaggle-data"
  }
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.gaggle.id
}
```

- [ ] **Step 2: Write variables.tf**

Create `infra/variables.tf`:

```hcl
variable "region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "instance_type" {
  description = "EC2 instance type (2GiB RAM so the web docker build doesn't OOM)"
  type        = string
  default     = "t3.small"
}

variable "root_volume_size" {
  description = "Root EBS volume size in GiB"
  type        = number
  default     = 20
}

variable "data_volume_size" {
  description = "Attached /data EBS volume size in GiB (docker data-root + volumes)"
  type        = number
  default     = 30
}

variable "admin_public_key" {
  description = "Admin SSH public key (owner access, also the aws_key_pair)"
  type        = string
}

variable "deploy_public_key" {
  description = "Public half of the deploy keypair that GitHub Actions uses to SSH in"
  type        = string
}
```

- [ ] **Step 3: Write outputs.tf and tfvars example**

Create `infra/outputs.tf`:

```hcl
output "public_ip" {
  description = "Public IP of the gaggle box (set as the DEPLOY_HOST secret)"
  value       = aws_instance.gaggle.public_ip
}

output "instance_id" {
  value = aws_instance.gaggle.id
}

output "data_volume_id" {
  value = aws_ebs_volume.data.id
}
```

Create `infra/terraform.tfvars.example`:

```hcl
region              = "us-east-1"
instance_type       = "t3.small"
root_volume_size    = 20
data_volume_size    = 30
admin_public_key    = "ssh-ed25519 AAAA... your-admin-key"
deploy_public_key   = "ssh-ed25519 AAAA... github-actions-deploy-key"
```

- [ ] **Step 4: Validate the configuration**

Run (from `infra/`):
```bash
nix shell nixpkgs#terraform --command terraform fmt -check
nix shell nixpkgs#terraform --command terraform init -backend=false
nix shell nixpkgs#terraform --command terraform validate
```
Expected: `fmt` clean, `init` downloads the aws provider, `validate` reports "Success!".

A real `terraform plan/apply` needs AWS credentials and is an **owner-run step** (Task 8 docs point at it) — not executable in this session.

- [ ] **Step 5: Commit**

```bash
git add infra/main.tf infra/variables.tf infra/outputs.tf infra/terraform.tfvars.example
git commit -m "feat: Terraform for the gaggle EC2 box"
```

---

### Task 8: Docs (README + .env.example) and owner handoff

**Files:**
- Modify: `README.md`, `.env.example`

**Interfaces:**
- Consumes: everything above; produces the runbook the owner follows to (a) provision AWS, (b) add GitHub secrets, (c) run the first deploy + smoke test.

- [ ] **Step 1: Update .env.example**

In `.env.example`, add a header comment above the `# API / auth` block:

```
# Production deploy: do NOT put real secrets here. They live as GitHub Actions
# secrets and are written to /srv/gaggle/.env on the EC2 box at deploy time
# (see README "Production deployment"). APP_ENV=production rejects these dev
# defaults at boot.
```

- [ ] **Step 2: Add the production section to README.md**

Append a "## Production deployment" section to `README.md`:

```markdown
## Production deployment

Gaggle deploys to a single AWS EC2 instance and is managed by GitHub Actions.

**Pipeline:** on push to `main`, `.github/workflows/ci.yml` runs `go vet` +
`go test` (against Postgres/Redis service containers) and the frontend
`lint`/`build`. Then `.github/workflows/deploy.yml` SSHes into the box
(`deploy@<ip>`) and runs `deploy/apply.sh`, which writes production secrets to
`/srv/gaggle/.env`, checks out the pushed commit, and runs
`docker compose -f compose.yaml -f compose.prod.yaml up --build -d`.

**Infra:** `infra/` is Terraform. From a machine with AWS credentials:

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars   # fill in your public keys
nix shell nixpkgs#terraform --command terraform init
nix shell nixpkgs#terraform --command terraform plan
nix shell nixpkgs#terraform --command terraform apply
```

The `public_ip` output is the box; set it as the `DEPLOY_HOST` secret.

**GitHub secrets required** (Settings → Secrets and variables → Actions):

| Secret | Value |
|---|---|
| `DEPLOY_HOST` | Public IP output from Terraform |
| `DEPLOY_SSH_KEY` | Private half of the keypair whose public half was passed as `deploy_public_key` to Terraform |
| `GAGGLE_JWT_SECRET` | Long random string (generate: `openssl rand -hex 32`) |
| `GAGGLE_DB_USER` | e.g. `gaggle` |
| `GAGGLE_DB_PASSWORD` | Long random string |
| `GAGGLE_DEPLOY_KEY` | Private half of a **GitHub repo deploy key** (Settings → Deploy keys, read-only) registered on `ba-reynolds/gaggle` — the box checks the repo out with it |

**First deploy + smoke test:** trigger `workflow_dispatch` on the Deploy
workflow, then verify: `ssh deploy@<ip> docker compose -f /srv/gaggle/compose.yaml -f /srv/gaggle/compose.prod.yaml ps` shows db/redis/api/web up; browse `http://<ip>`; sign up a test user; post with media; `docker compose restart` and confirm posts + media persist (EBS-backed).

**Current limitations (pilot):** served over plain HTTP — auth cookies and the
refresh-token cookie travel in cleartext and `COOKIE_SECURE` is `false`. Buy a
domain and this moves to TLS (ACM cert + nginx + `COOKIE_SECURE=true`) as a
follow-up. db/redis are not exposed outside the box.
```

- [ ] **Step 3: Verify gitignore covers the new prod artifacts**

Run: `git check-ignore .env server/.env 2>/dev/null; git ls-files | rg 'dockerignore|\.gitignore'`
Expected: check-ignore prints `.env` and `server/.env` (both ignored); no `deploy/.env.production.template` or secret caught by ignore rules.

- [ ] **Step 4: Commit**

```bash
git add README.md .env.example
git commit -m "docs: production deployment runbook"
```

---

## Post-implementation verification (owner, after this plan completes)

1. `terraform apply` in `infra/` (needs AWS creds).
2. Add the GitHub secrets from the README table.
3. First deploy via `workflow_dispatch`; confirm the smoke test steps in Task 8.