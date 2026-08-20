# Faster Agent Loop — Design

## Context

Agents currently pay `make proj-dev` → `scripts/proj-up.sh gaggle-<slug>` → `docker compose -p <proj> up --build -d --wait` for every change, even frontend-only edits. Measured on `login-goose-logo`: `go build` ~11s + `migrate` install ~8s + `vite build` ~4.5s + layer export + sequential health checks (~10s) = 30–60s warm, minutes cold. The script also fails on DB/Redis host-port collisions (only `API_PORT/WEB_PORT` are hashed). The TUI's `Build · Muse Spark · 4m55s` is session-elapsed, not per-command, but the Docker wait is real. Goal: sub-10s feedback for frontend-only changes; avoid rebuilding the unchanged image; auto-allocate DB/Redis ports.

## Decisions (confirmed)

- Frontend-only work (web/**, web/public/**, web/Dockerfile, web/index.html, compose.yaml web section) should default to **host Vite HMR** (`proj-dev-fe`/`dev-frontend` style) proxied to the shared or isolated API, with an explicit opt-in to the full `proj-dev` (nginx/TLS) when needed.
- `proj-up.sh` should auto-allocate `DB_PORT`/`REDIS_PORT` the same way it does `API_PORT`/`WEB_PORT` (hash + persist to `~/.local/state/gaggle-proj/<proj>.env`), eliminating manual `DB_PORT=6970 REDIS_PORT=6380`.
- `proj-up.sh` should do **selective builds**: if only `web/**` touched, `docker compose build web` only (and vice versa for `server/**`); otherwise build both. Use `git diff --name-only` against `main` or prior commit when in a worktree.
- `new-task.md` should branch: UI-visible → `make proj-dev-fe` (or `proj-dev` if nginx-sensitive); docs/backend-only → `npm run build`/`npm run lint` inside `web-tools`/`tools` without a preview stack.

## Architecture

### Components

1. **`scripts/proj-up.sh`** — extended to: (a) persist `DB_PORT`/`REDIS_PORT` alongside `API_PORT`/`WEB_PORT`; (b) accept `BUILD_MODE` env (`full`/`web-only`/`api-only`/`auto`); (c) in `auto`, detect changed paths and `docker compose build` only the needed service(s) before `up -d --wait`. Times each phase (`date +%s`) so real durations print.
2. **`Makefile`** — new targets:
   - `proj-dev-fe` / `proj-dev-frontend` — isolated `db`+`redis`+`api` (if needed) + host Vite (`cd web && npm run dev -- --host --port $WEB_PORT`), no nginx build. Documented as the frontend default.
   - `proj-dev-web-only`, `proj-dev-api-only` — explicit selective builds forwarding `BUILD_MODE`.
   - Keep `proj-dev` as alias to `proj-dev-full` (current behavior) for backwards compat.
3. **`.opencode/commands/new-task.md`** — update step 6 to explain fast vs full preview choice and when to skip Docker entirely (backend/docs).
4. **No compose.yaml schema change** — `DB_PORT`/`REDIS_PORT` already templated (`${DB_PORT:-6969}`), just need values.

### Data flow

- Worktree → `git diff --name-only origin/main...HEAD` (or `HEAD~1` if single commit) → `BUILD_MODE` → `docker compose -p $PROJ build <service(s)>` → `docker compose -p $PROJ up -d --wait`.
- Host Vite path: `vite.config.ts` already proxies `/api, /swagger → http://localhost:2021` (shared) and can target isolated `API_PORT` via `VITE_API_BASE_URL` or env.

### Error handling

- Port allocation: `port_in_use()` probe loop (already in script) extended to `DB_PORT`/`REDIS_PORT`; on collision bump +1.
- Selective build fallback: if `git` unavailable, build both.
- Health waits remain `--wait`; `proj-dev-fe` waits only for `db`/`redis`/`api`, not `web`.

## Testing

- Manual: (a) frontend-only change (`web/src/pages/LoginPage.tsx`) → `make proj-dev-fe` → HMR + `curl http://localhost:$WEB_PORT` 200; (b) `web/public/gaggle-goose.png` change → `web-only` build still serves new asset (curl `gaggle-goose.png` hash); (c) `server/**` change → `api-only` path rebuilds api; (d) concurrent second worktree → auto `DB_PORT` distinct, no bind error.
- Lint/build: `docker compose --profile tools run --rm web-tools npm run lint` / `npm run build` still pass; `go test ./...` unchanged.

## Risks

- Host Vite diverges from nginx (TLS/proxy rules) — mitigated by documenting when to use full preview (nginx.conf, docker-entrypoint, 403/healthcheck cases). See `deploy/apply.sh` and `compose.yaml:122-143` healthcheck rationale.
- `git diff` heuristics in detached worktrees — fallback to full build.

## Follow-up

- Add `scripts/proj-up-fe.sh` or fold into `proj-up.sh --fe`.
- Persist timings to `~/.local/state/gaggle-proj/*.timing` for future offender ranking.

## Approval

- User approved faster changes 2026-08-20 (build mode). Bounded vs architectural: treated as architectural-lite with this spec as the gate; implementation follows in next turn.
