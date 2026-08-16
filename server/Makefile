include .env
export PGUSER=$(DB_USER)
DB_URI = "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_ADDRESS)/$(DB_NAME)?sslmode=disable"
MIGRATE := go run -tags=postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

# The `filter-out` at the end is just going to give us "xyz" for when we run "make new-migration xyz".
# This gives an error when running the command as make will try to parse "xyz" as if it was another rule
# defined in this file
new-migration:
	@$(MIGRATE) create -seq -ext sql -dir cmd/migrate/migrations $(filter-out $@,$(MAKECMDGOALS))

migrate-up:
	@$(MIGRATE) -path ./cmd/migrate/migrations -database=$(DB_URI) up

migrate-down:
	@$(MIGRATE) -path ./cmd/migrate/migrations -database=$(DB_URI) down

migrate-force:
	@$(MIGRATE) -path ./cmd/migrate/migrations -database=$(DB_URI) force $(filter-out $@,$(MAKECMDGOALS))

check-db:
	psql -h localhost -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME)

reset-db:
	@echo "Resetting database $(DB_NAME)..."
	dropdb -h localhost -p $(DB_PORT) -U $(DB_USER) --if-exists $(DB_NAME)
	createdb -h localhost -p $(DB_PORT) -U $(DB_USER) $(DB_NAME)
	@echo "Database $(DB_NAME) has been reset."

seed-db:
	go run cmd/seed/main.go

swag:
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go --parseDependency --output docs

run:
	go run cmd/api/main.go

# Testing targets. By default these run against a local Postgres/Redis started
# via `make dev-services`; override with TEST_DATABASE_URL / TEST_REDIS_ADDR.
test:
	go test ./... -v

test-coverage:
	go test ./... -cover

test-coverage-report:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-short:
	go test ./... -v -short

# Start local dev services (Postgres + Redis) without Docker.
dev-services:
	@scripts/dev-services.sh start

dev-services-stop:
	@scripts/dev-services.sh stop

dev-services-status:
	@scripts/dev-services.sh status

# catch-all rule for composed rules such as `migrate-force` and `new-migration`
%:
	@:
