# Gaggle

A Twitter-like social media app: **Go** (chi + PostgreSQL + Redis) backend and a
**React + TypeScript + shadcn/ui** (Vite) frontend, served by **nginx**. Single
repository, two apps, fully containerized with Docker Compose.

## Features

- JWT auth (access + refresh tokens, refresh-token rotation via HttpOnly cookie)
- Posts with text + up to 4 images, replies, quotes, reposts
- Likes, reposts, quotes, bookmarks (with per-user bookmark categories)
- Home feed (following), user feeds, liked / bookmarked feeds
- Feed of users who **liked / reposted** a post, and **quotes** of a post
- Profiles with banner/avatar upload, follower/following counts, follow/block
- User settings (notifications, privacy, appearance, language)
- Cursor-based (keyset) pagination on every feed
- Redis: home-feed caching + auth rate limiting (degrades gracefully if absent)
- In-app notifications for likes, reposts, quotes, replies, follows, and mentions
- Live notification/feed events over cookie-authenticated Server-Sent Events (SSE)
- Full-text post and user search, clickable hashtags, hashtag feeds, and trends
- Post power features: edit with history, pin to profile (one per author),
  cascade delete (post + all replies), and opinion polls (top-level only,
  2-4 options, one vote per user)
- Profile badges: auto-earned milestone badges (account age, posts, followers,
  likes received) plus admin-assigned badges managed from an admin UI
- Admin area at `/admin` (only for admins; `alice` is the seeded admin)
- Explore page at `/explore`: search, trending hashtags, and suggested users
  ("Who to follow" in the sidebar is backed by the same suggested endpoint)
- User-managed lists at `/lists`: public, owner-curated collections of users
  with their own aggregated feed
- Direct messages at `/messages`: 1:1 private conversations delivered over the
  SSE stream with unread badges; blocked users cannot message
- Consistent `{data, error}` JSON envelope on every endpoint
- Swagger UI at `/swagger` (regenerate with `make swag`)

## Repository layout

```
server/    Go API server (handlers -> services -> stores -> PostgreSQL) + Dockerfile
web/       React + Vite + shadcn/ui client + nginx reverse proxy (Dockerfile)
compose.yaml  One-shot local stack: Postgres + Redis + API + Web
```

## Quick start (Docker)

**Prerequisite:** Docker Engine running (`docker --version`). On NixOS that's
`virtualisation.docker.enable = true` + `nixos-rebuild switch` + reboot.

```bash
make dev       # build + start db, redis, api, and web
make seed      # create demo users (idempotent)
```

Open **http://localhost:5173** and sign in as `alice@example.com` / `password123`.

- The `web` container (nginx on host `:5173`) serves the frontend and
  reverse-proxies `/api/*` and `/swagger/*` to the `api` container — the whole
  app runs on a single origin (no CORS).
- The `api` container **auto-applies migrations** on first start and seeds
  nothing by itself; run `make seed` once for demo data.

### Common commands

| Command | What it does |
|---------|--------------|
| `make dev` | build + run the full stack in the background |
| `make dev-logs` | stream all container logs |
| `make dev-stop` | stop the stack (data volumes persist) |
| `make seed` | create demo users (idempotent) |
| `make test` | backend integration tests (throwaway `social_test` DB) |
| `make swag` | regenerate Swagger docs into `server/docs` |
| `make lint-frontend`, `make test-frontend` | frontend checks |
| `make reset-db` | drop + recreate the app database |

No Go/Node toolchain is needed on the host — everything runs through the
`tools` / `web-tools` compose services (a `tools` profile not started by
`make dev`).

Notifications are available at `/notifications`; the SSE stream is exposed at
`/api/v1/stream` and is consumed automatically by the frontend. The stream
uses the same-origin HttpOnly refresh cookie, so no token is placed in URLs.

## Configuration

Copy `.env.example` → `.env` to override compose variables (DB user/password,
ports, JWT secret). Sensible defaults work out of the box. The API container's
full config lives in `compose.yaml` under the `api` service `environment`.

## API contract

Every response is an envelope:

```json
{ "data": <payload>, "error": null }
{ "data": null, "error": { "code": "NOT_FOUND", "message": "..." } }
```

Paginated feeds return `{ "items": [...], "next_cursor": ..., "has_more": bool }`.
Post objects expose counts and the requesting user's state under `engagement`
(`is_liked`, `is_reposted`, `is_bookmarked`, `like_count`, ...).
See `server/docs/swagger.yaml` for the full spec.

## Notes

- Media files are served publicly at `/api/v1/media/{uuid}` (UUIDs are
  unguessable); `<img>` tags can't send auth headers.
- The backend degrades gracefully when Redis is down (no caching / rate limiting).
- Migrations: applied automatically by the api container on start; manage
  manually with `make migrate-up` / `make new-migration <name>`.

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

Commit `infra/.terraform.lock.hcl` once it appears after `terraform init`
(provider version pinning). After `terraform apply`, add the GitHub secrets
from the table below before triggering the first `workflow_dispatch` deploy.

The `public_ip` output is the box; set it as the `DEPLOY_HOST` secret.

**If you're new to this — exact commands (in order):**

1. **Generate keys + secrets** (idempotent; skip if you already have them)

   ```bash
   bash scripts/provision.sh            # creates ~/.gaggle-deploy/ keys + secrets.env
   cat ~/.gaggle-deploy/gaggle_admin.pub        # -> admin_public_key
   cat ~/.gaggle-deploy/gaggle_ci.pub           # -> deploy_public_key
   cat ~/.gaggle-deploy/gaggle_repo_deploy.pub  # -> GitHub repo deploy key blob
   ```

2. **Register the repo deploy key on GitHub** — repo → Settings → Deploy keys
   → **Add deploy key**: title `gaggle-box`, paste the
   `gaggle_repo_deploy.pub` contents, read-only is fine.

3. **Provision the box**

   ```bash
   cd infra
   cp terraform.tfvars.example terraform.tfvars
   # in terraform.tfvars: paste the two public keys from step 1 verbatim
   nix shell nixpkgs#terraform --command terraform init
   nix shell nixpkgs#terraform --command terraform apply   # note the public_ip output
   ```

4. **Add the GitHub secrets** — repo → Settings → Secrets and variables →
   Actions → **New repository secret**, once per row of the table below. The
   two private keys (`DEPLOY_SSH_KEY` = `gaggle_ci`, `GAGGLE_DEPLOY_KEY` =
   `gaggle_repo_deploy`) paste as multi-line values — keep the blank first line
   as-is, it is the OpenSSH header. If you prefer, `scripts/provision.sh` can
   set all six for you with the `gh` CLI (just run it with `DEPLOY_HOST=...
   bash scripts/provision.sh`).

5. **Deploy** — wait a few minutes for first-boot provisioning (packages,
   Docker, EBS format) to finish on the box. Then repo → **Actions** →
   **Deploy** → **Run workflow** (on `main`).

6. **Smoke test** — browse `http://<public_ip>` (or `https://<public_ip>` —
   accept the self-signed cert warning); sign up a test user; post with
   media; then on the box `docker compose -f /srv/gaggle/compose.yaml -f
   /srv/gaggle/compose.prod.yaml restart` and confirm posts + media persist
   (they're EBS-backed).

If the first Deploy run fails, likely causes are: you ran it before the box
finished first-boot provisioning (wait, re-run), or `DEPLOY_HOST`/secrets are
missing (check the run's log for `DEPLOY_HOST secret not set`).

**GitHub secrets required** (Settings → Secrets and variables → Actions):

| Secret | Value |
|---|---|
| `DEPLOY_HOST` | Public IP output from Terraform |
| `DEPLOY_SSH_KEY` | Private half of the keypair whose public half was passed as `deploy_public_key` to Terraform |
| `GAGGLE_JWT_SECRET` | Long random string (generate: `openssl rand -hex 32`) |
| `GAGGLE_DB_USER` | e.g. `gaggle` |
| `GAGGLE_DB_PASSWORD` | Long random string |
| `GAGGLE_DEPLOY_KEY` | Private half of a **GitHub repo deploy key** (Settings → Deploy keys, read-only) registered on `ba-reynolds/gaggle` — the box checks the repo out with it. Its public half is NOT the `deploy_public_key` baked into the box's bootstrap; that keypair is only for GitHub Actions SSHing in as `deploy@` |
| `GAGGLE_HTTPS_DOMAIN` (optional) | Domain pointing at the box's public IP; enables the certbot service to provision a real Let's Encrypt cert on 443. Omit/empty to keep the self-signed fallback. |

**First deploy + smoke test:** trigger `workflow_dispatch` on the Deploy
workflow, then verify: `ssh deploy@<ip> docker compose -f /srv/gaggle/compose.yaml -f /srv/gaggle/compose.prod.yaml ps` shows db/redis/api/web up; browse `http://<ip>`; sign up a test user; post with media; `docker compose restart` and confirm posts + media persist (EBS-backed). HTTPS on 443 is live (self-signed cert) from the start. The deploy itself now rebuilds the images uncached and health-checks the fixed-name public assets (`/favicon.ico`, `/gaggle-goose.png`) so a stale/corrupt `web` layer can't silently 403 the favicon + sidebar logo again.

Sessions: the refresh-token cookie's `Secure` flag follows the scheme the
browser actually used (via nginx `X-Forwarded-Proto`), so `http://<ip>` keeps a
persistent session even though `COOKIE_SECURE=true` is set for direct-API
fallback; HTTPS clients still get a Secure cookie. Refresh-token rotation
treats an already-rotated token replayed from the *same device* (user-agent) as
the benign multi-tab/stale-cookie case and keeps the session alive; replay from
a different device is still treated as theft and revokes the session family.

If the favicon/logo still 403 after a deploy (stale filesystem inside the
running container), recreate the web container explicitly:
`ssh deploy@<ip> 'cd /srv/gaggle && docker compose -f compose.yaml -f compose.prod.yaml up -d --force-recreate web'`.

**Current state:** the app is served over TLS. The box serves **HTTPS on
port 443** (already open in the security group + host firewall) using a
self-signed certificate by default, so `https://<public-ip>` works from any
browser (it will show one warning until you accept the cert). Plain **HTTP
on port 80** still works, which the ACME (Let's Encrypt) HTTP-01 validation
uses, and keeps the box reachable while no domain exists.

**Buying a domain → real cert (minimal changes):**

1. Point a DNS **A record** at the box's public IP (any registrar — no extra
   AWS setup needed).
2. Set the `GAGGLE_HTTPS_DOMAIN` GitHub secret to your domain (e.g.
   `gaggle.example.com`) and deploy. Alternatively run the certbot image once
   on the box directly (see below). The compose `certbot` service auto-runs
   `certbot certonly --webroot` on start and renews on a 12h loop.
3. Once the cert is issued, nginx serves the real cert on 443 (the
   self-signed fallback is only used when no cert exists yet).
4. Optional but recommended: switch health checks / traffic to HTTPS only and
   add an HTTP→HTTPS redirect in `web/nginx.conf` (out of scope to keep HTTP
   working during the no-domain pilot).

Manual one-shot issuance on the box (if you skipped the GitHub secret):

```bash
ssh deploy@<ip>
cd /srv/gaggle
docker compose run --rm certbot \
  certbot certonly --cert-name gaggle --webroot -w /var/www/certbot \
  -d gaggle.example.com --non-interactive --agree-tos -m you@example.com
docker compose restart web certbot
```

Renewals run automatically on the 12h loop; after a renewal the `web`
container needs a reload to serve the new cert, e.g.
`docker compose restart web` (also applied on every deploy).

db/redis are not exposed outside the box.
