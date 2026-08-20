.PHONY: dev dev-stop dev-logs seed migrate-up reset-db test test-backend test-coverage test-frontend lint-frontend swag build build-backend build-frontend proj-dev proj-stop proj-logs proj-ps proj-seed simulate help

# ---- One-shot stack ----

# Build images and start the whole stack (db + redis + api + web).
dev:
	docker compose up --build -d
	@docker compose ps
	@echo
	@echo "  Frontend: http://localhost:5173        (HTTPS on 443: WEB_HTTPS_PORT=8443 make dev)"
	@echo "  API:      http://localhost:2021   (swagger: /swagger)"
	@echo "  Seed:     make seed   (creates alice@example.com / password123)"
	@echo "  Stop:     make dev-stop"

# Stop (and remove) containers; keeps named volumes.
dev-stop:
	docker compose down

# ---- Isolated per-worktree preview stack ----
# Run from INSIDE an agent-branch/<slug> worktree. The slug is derived from
# the directory name, so each worktree gets its own project (gaggle-<slug>)
# with private containers, volumes, and ports — see scripts/proj-up.sh.
# Do NOT use these from the repo root (that would preview the master stack).

PROJ := gaggle-$(notdir $(CURDIR))

proj-dev:
	@scripts/proj-up.sh $(PROJ)

# Selective builds: only rebuild the service whose sources changed.
proj-dev-web-only:
	@BUILD_MODE=web-only scripts/proj-up.sh $(PROJ)

proj-dev-api-only:
	@BUILD_MODE=api-only scripts/proj-up.sh $(PROJ)

# Skip image builds entirely (reuse last images) — fastest re-up.
proj-dev-nobuild:
	@BUILD_MODE=none scripts/proj-up.sh $(PROJ)

# Fast frontend loop: isolated db/redis/api + host Vite HMR (no nginx build).
# Uses the same DB/REDIS ports as proj-dev but skips the web image.
proj-dev-fe:
	@scripts/proj-up.sh --fe-only $(PROJ) 2>/dev/null || (echo ">> proj-dev-fe: starting db/redis/api only (host Vite handles web)" && docker compose -p $(PROJ) up --build -d --wait db redis api && echo "" && . ~/.local/state/gaggle-proj/$(PROJ).env 2>/dev/null; echo "  API: http://localhost:$${API_PORT:-2021}   (swagger: /swagger)"; echo "  Next: cd web && npm run dev -- --host --port $${WEB_PORT:-5173}  (proxies /api → api)" && echo "  Or: make proj-dev  for full nginx preview")

proj-stop:
	docker compose -p $(PROJ) down

proj-logs:
	docker compose -p $(PROJ) logs -f

proj-ps:
	docker compose -p $(PROJ) ps

proj-seed:
	docker compose -p $(PROJ) run --rm --no-deps --entrypoint /app/seed api

# Stream logs from all services.
dev-logs:
	docker compose logs -f

# ---- Backend ----

# Seed demo users (idempotent). Runs the seed binary inside the api image.
seed:
	docker compose run --rm --no-deps --entrypoint /app/seed api

# Simulate one tick of live user activity (posts/replies/likes/DMs) on top of
# the seeded data. Timestamps near now(), so the community grows over time.
# Local analogue of the scheduled cron on the EC2 box.
simulate:
	docker compose run --rm --no-deps --entrypoint /app/simulate api

# Apply migrations manually (the api container also auto-migrates on start).
migrate-up:
	docker compose run --rm --no-deps api /app/migrate -path /app/migrations -database "postgres://white:teeth@db:5432/social?sslmode=disable" up

# Backend tests against the Postgres container (uses a throwaway social_test DB).
test: test-backend
test-backend:
	docker compose --profile tools run --rm tools go test ./...

test-coverage:
	docker compose --profile tools run --rm tools go test ./... -cover

# Drop + recreate the app database (data loss!). Uses the db container's psql.
reset-db:
	docker compose exec db psql -U white -d postgres -c "DROP DATABASE IF EXISTS social;" 
	docker compose exec db psql -U white -d postgres -c "CREATE DATABASE social;"

# Regenerate swagger docs (writes to server/docs).
swag:
	docker compose --profile tools run --rm tools sh -c 'go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go --parseDependency --output docs'

# ---- Frontend ----

dev-frontend:
	@echo ">> local vite dev needs the API reachable; prefer 'make dev' (full stack in Docker)"
	cd web && npm install && npm run dev

test-frontend:
	docker compose --profile tools run --rm web-tools npm ci
	docker compose --profile tools run --rm web-tools npm run build

lint-frontend:
	docker compose --profile tools run --rm web-tools npm ci
	docker compose --profile tools run --rm web-tools npm run lint

# ---- Everything ----

build:
	docker compose build

build-backend:
	docker compose build api

build-frontend:
	docker compose build web

help:
	@echo "Gaggle Docker targets:"
	@echo "  make dev         build + run full stack (db redis api web)"
	@echo "  make dev-stop    stop the stack"
	@echo "  make dev-logs    stream logs"
	@echo "  make seed        seed demo data (idempotent; also runs on start)"
	@echo "  make simulate    one tick of live activity (posts/replies/likes/DMs)"
	@echo "  make test        backend tests (throwaway social_test DB)"
	@echo "  make swag        regenerate swagger docs"
	@echo "  make lint-frontend / test-frontend"
