# Gaggle: GitHub CI/CD + AWS EC2 deployment — Design

Date: 2026-08-19
Status: Approved in chat on 2026-08-19

## Goal

Give Gaggle a CI/CD pipeline on GitHub and deploy it to production on a single
AWS EC2 instance running the existing Docker Compose stack. CI runs lint +
tests on every PR; pushing to `main` runs the same checks then automatically
deploys to the box. The box is provisioned with Terraform.

This builds on the prior analysis spike (`agent/cloud-deploy-email-analysis`),
which found the stack is already cloud-friendly: two custom Dockerfiles,
100% env-var-driven config, static cgo-free Go binaries, migrations applied on
container start.

## Decisions (confirmed with the owner)

- **Compute:** EC2 + Docker Compose (not ECS/App Runner/Lightsail). Closest to
  local dev, cheapest.
- **Data tier:** all-in-one on the EC2 box (Postgres + Redis as compose
  services), not managed RDS/ElastiCache. Data lives on an attached EBS volume.
- **Domain/HTTPS:** none yet; deploy on the public IP over plain HTTP as a
  pilot. TLS + `COOKIE_SECURE=true` + ACM cert are a deferred follow-up once a
  domain is bought.
- **CI/CD:** auto-deploy on push to `main`, automated `workflow_dispatch`
  redeploy, check-only CI on PRs.
- **Infra tooling:** Terraform (managed by the user from NixOS via
  `nix shell nixpkgs#terraform`).
- **Secrets:** GitHub Actions secrets; the deploy workflow writes `.env` on the
  box. No shipped production defaults.
- **Deploy transport:** GitHub Actions SSHes into the box, checks out the
  target commit, and runs `docker compose up --build -d` on the box (no ECR).
- **SSH exposure:** port 22 open to the internet, key-only auth (no passwords),
  UFW firewall, fail2ban.

## Architecture

```
[ GitHub repo: ba-reynolds/gaggle ]
        │  push to main
        ▼
[ GitHub Actions ]
   └─ ci.yml    PR:  backend vet+test, frontend lint+build
   └─ deploy.yml main: same checks, then deploy job
        │  ssh (DEPLOY_SSH_KEY secret) -> deploy@<public-ip>
        ▼
[ EC2 t3.small (Amazon Linux 2023) ]
   ├─ Docker Engine + Compose plugin (installed by bootstrap.sh user-data)
   ├─ git checkout of <sha> (deploy user, read-only deploy key)
   ├─ /data/...  EBS data volume (root of docker data-root + repo lockfiles)
   └─ docker compose -f compose.yaml -f compose.prod.yaml up --build -d
        ├─ db     postgres:16-alpine   (NOT published to host)
        ├─ redis  redis:7-alpine       (NOT published to host)
        ├─ api    gaggle api on :2021  (internal only)
        └─ web    nginx on :80         (proxies /api + /swagger + /assets)
        │
        ▼
[ http://<public-ip>:80 ]  →  single origin, same as local dev
```

Topology is identical to local dev except: db/redis ports are NOT exposed to
the host (removed in `compose.prod.yaml`), `LOGGING_FILENAME` points at
stdout in prod (so logs are collected by the OS journal/cloudwatch agent later),
and secrets come from `.env` written by the deploy, never from repo defaults.

## Repo changes

New files:

| Path | Purpose |
|---|---|
| `.github/workflows/ci.yml` | Backend vet+test (postgres:16 + redis:7 service containers) and frontend lint+build, on every PR and push. |
| `.github/workflows/deploy.yml` | Runs on push to `main` (+ `workflow_dispatch`). Reuses the same test job, then deploys. |
| `deploy/apply.sh` | Runs on the box at deploy time: write `.env.` from env vars, checkout the target SHA, `docker compose -f compose.yaml -f compose.prod.yaml up --build -d`, health-check, rollback note. |
| `deploy/.env.production.template` | Documented shape of `.env` written by the deploy. |
| `compose.prod.yaml` | Prod compose override. No db/redis `ports:`, cookies stay non-secure for now (documented), env values overridden from `.env`. |
| `infra/main.tf` | Terraform: provider, key pair, security group, instance, EBS volume + attachment. |
| `infra/variables.tf` | Tunables: region, instance type (default `t3.small`), AMI, admin public key, deploy public key, data volume size. |
| `infra/outputs.tf` | Public IP, instance id, data volume id. |
| `infra/bootstrap.sh` | User-data: install Docker + compose plugin, create `deploy` user, install deploy + admin SSH keys, UFW (22/80/443), fail2ban, mount `/data` EBS, set docker `data-root` to `/data/docker`, create `/srv/gaggle` checkout owned by `deploy`. |

Modified files:

| Path | Change |
|---|---|
| `.env.example` | Note that production secrets are NOT committed; prod uses GitHub Actions secrets. |
| `README.md` | "Production deploy" section: pointers to `infra/`, `deploy/`, the two workflows, prerequisites (AWS creds, terraform), and the pilot limitations (no TLS). |

The API keeps its dev-default `JWT_SECRET` for `make dev` only (unchanged
behavior locally); the prod compose override sets a real secret from `.env` and
the app fails to boot without it in prod. If `JWT_SECRET` is unset in prod,
`docker-entrypoint.sh`/config should fail fast rather than fall back to the dev
secret — implemented as a config check when `GIN_MODE`/`APP_ENV=production` is
set (see below, this is the one small code change).

### Code change: fail-fast secrets in production

`server/pkg/config/config.go` currently defaults `JWT_SECRET` to `"secret"`.
In production (detected by a new `APP_ENV=production` env var set in
`compose.prod.yaml`), `LoadConfig` returns an error if `JWT_SECRET` or
`DB_PASSWORD` are empty or equal to the known dev defaults, so an unset secret
cannot silently boot a production instance with the shipping default.

## GitHub Actions workflows

### ci.yml — every PR and push to any branch

- **backend** job (ubuntu runner):
  - service containers: `postgres:16-alpine` (ports 5432) and `redis:7-alpine`.
  - `setup-go` with go 1.24 + cache.
  - `go vet ./...`
  - `go test ./...` with `TEST_DB_ADDRESS=localhost:5432`, `TEST_DB_USER=white`,
    `TEST_DB_PASSWORD=teeth`, `TEST_DB_NAME=social_test` — mirrors the local
    `tools` + `testutil` wiring, so the same throwaway-db integration tests run.
- **frontend** job:
  - `setup-node` node 24 + npm cache.
  - `npm ci` (honors `allowScripts` in package.json).
  - `npm run lint`
  - `npm run build`

### deploy.yml — push to `main` (and `workflow_dispatch`)

- Reuses the same backend + frontend check jobs (via `workflow_call` or
  duplicated steps; prefer a shared `ci.yml` called from `deploy.yml`).
- **deploy** job (`needs: [backend, frontend]`), `concurrency` group locked per
  environment so two pushes can't race the box:
  - `SSH into deploy@${{ secrets.DEPLOY_HOST }}` and run `deploy/apply.sh` with
    environment variables from secrets: `GAGGLE_JWT_SECRET`,
    `GAGGLE_DB_PASSWORD`, `DEPLOY_KEY` (for the git checkout), `TARGET_SHA`.
  - Health check: `curl -f http://localhost/swagger/doc.json` on the box after
    `up --build -d`; non-zero exit marks the job failed.
- GitHub Actions secrets required: `DEPLOY_HOST` (public IP or DNS),
  `DEPLOY_SSH_KEY` (private key for `deploy@`), `GAGGLE_JWT_SECRET`,
  `GAGGLE_DB_PASSWORD`.

## deploy/apply.sh behavior

Runs on the box, `set -euo pipefail`:

1. Checkout the target SHA in `/srv/gaggle` (`git fetch origin main`, hard
   reset to `TARGET_SHA`).
2. Write `.env` for production from the injected vars (cookies non-secure,
   DB/REDIS hostnames point at compose service names, keep sslmode=disable for
   the same-host DB).
3. `docker compose -f compose.yaml -f compose.prod.yaml build` then `up -d`.
4. `docker compose ps` and a curl health check to `/swagger/doc.json`; on
   failure, print `docker compose logs --tail=100 api` and exit 1 (a rollback
   to the previous SHA is a manual `git checkout <prev>` + re-run, documented in
   the script's comments).

## Terraform (`infra/`)

Applied once by the owner. Resources:

- `aws_key_pair` from `var.admin_public_key` — used for the admin SSH user's
  authorized_keys during bootstrap (owner access). Deploy key is added by
  bootstrap from a public key the owner pastes in, which GitHub's
  `DEPLOY_SSH_KEY` private half matches.
- `aws_security_group` `gaggle-sg`:
  - egress all allowed
  - ingress: `tcp/22` from `0.0.0.0/0`, `tcp/80` from `0.0.0.0/0`,
    `tcp/443` from `0.0.0.0/0` (443 forward-looking for TLS follow-up).
- `aws_instance` `gaggle-web`:
  - Amazon Linux 2023, `t3.small` (2 vCPU / 2 GiB — enough headroom to build
    the web image in CI's place; `t3.micro` would OOM the web build).
  - root EBS `gp3` 20 GiB.
  - `user_data` = rendered `infra/bootstrap.sh` with variables substituted.
  - `aws_ebs_volume` `gp3` 30 GiB attached at
    `/dev/sdf`, mounted at `/data` by bootstrap.
- `outputs`: `public_ip`, `instance_id`.

`bootstrap.sh` (idempotent, so Terraform re-runs are safe):

1. Install Docker Engine + compose plugin from Amazon's package repos.
2. `systemctl enable --now docker`.
3. Create `deploy` user; install deploy + admin public keys into
   `/home/deploy/.ssh/authorized_keys`; disable password auth + root login in
   sshd.
4. Set up UFW: allow 22, 80, 443; enable.
5. Install + enable fail2ban (default sshd jail).
6. Format (if unformatted) + mount `/data`; move Docker `data-root` to
   `/data/docker` via `/etc/docker/daemon.json`; restart docker.
7. `git clone` the repo into `/srv/gaggle` owned by `deploy` (idempotent:
   skip if present).

## Security posture summary

- No root SSH; key-only auth; `PasswordAuthentication no`.
- UFW: 22/80/443 only.
- fail2ban default sshd jail.
- db/redis not published to the host in prod (`compose.prod.yaml` removes
  `ports:`), reachable only over the internal compose network.
- Secrets never in the repo or the box's initial clone; written by deploys.
- **Known limitation (documented, accepted for the pilot):** no TLS → auth and
  refresh cookies travel in cleartext over the public internet, and media is
  served over plain HTTP. Resolved as a follow-up when a domain is purchased:
  ACM cert + nginx TLS + `COOKIE_SECURE=true` + DNS.

## Testing / verification

- CI green on PRs (vet, tests, lint, build) → main merges are always deployable.
- `workflow_dispatch` deploy lets the owner redeploy current `main` manually.
- Deploy job fails loudly on a bad build, failing migrate, or unhealthy API.
- Manual smoke after first deploy: `docker compose ps` shows 4 healthy
  services; browse `http://<public-ip>`; sign up a test user; post with media;
  restart the stack (`docker compose restart`) and confirm media + posts
  persist (EBS-backed volumes).

## Out of scope (deferred)

- Domain purchase, DNS, ACM cert, TLS, `COOKIE_SECURE=true`.
- Managed RDS / ElastiCache.
- Media to S3.
- Email verification (covered by `agent/cloud-deploy-email-analysis`).
- Google OAuth (covered by `agent/google-oauth-analysis`).
- GitHub branch protection / CODEOWNERS.

## Open action for the owner

Before first deploy, the owner must provide: an AWS account + credentials for
`terraform apply`, an admin SSH public key, a deploy SSH keypair (public half
goes to Terraform/bootstrap, private half to the `DEPLOY_SSH_KEY` GitHub
secret), and the GitHub secrets listed above.