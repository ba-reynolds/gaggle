.PHONY: dev dev-stop dev-logs seed migrate-up reset-db test test-backend test-coverage test-frontend lint-frontend swag build build-backend build-frontend help

# ---- One-shot stack ----

# Build images and start the whole stack (db + redis + api + web).
dev:
	docker compose up --build -d
	@docker compose ps
	@echo
	@echo "  Frontend: http://localhost:5173"
	@echo "  API:      http://localhost:2021   (swagger: /swagger)"
	@echo "  Seed:     make seed   (creates alice@example.com / password123)"
	@echo "  Stop:     make dev-stop"

# Stop (and remove) containers; keeps named volumes.
dev-stop:
	docker compose down

# Stream logs from all services.
dev-logs:
	docker compose logs -f

# ---- Backend ----

# Seed demo users (idempotent). Runs the seed binary inside the api image.
seed:
	docker compose run --rm --no-deps --entrypoint /app/seed api

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
	@echo "GopherSocial Docker targets:"
	@echo "  make dev         build + run full stack (db redis api web)"
	@echo "  make dev-stop    stop the stack"
	@echo "  make dev-logs    stream logs"
	@echo "  make seed        create demo users (idempotent)"
	@echo "  make test        backend tests (throwaway social_test DB)"
	@echo "  make swag        regenerate swagger docs"
	@echo "  make lint-frontend / test-frontend"
