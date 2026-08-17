# Go tooling targets for the server. All go commands run inside the Docker
# `tools` container (see ../compose.yaml) so nothing needs to be installed on
# the host. Run from the repo root for the compose-aware stack:
#   make migrate-up
#   make new-migration add_users
#   make test

MIGRATE_ARGS = -path /src/cmd/migrate/migrations -database "postgres://white:teeth@db:5432/social?sslmode=disable"

# Create a new pair of migration files:  make new-migration add_foo
new-migration:
	docker compose --profile tools run --rm tools go run -tags=postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 create -seq -ext sql -dir /src/cmd/migrate/migrations $(filter-out $@,$(MAKECMDGOALS))

migrate-up:
	@echo ">> the api container auto-migrates on start; use this to migrate manually:"
	docker compose --profile tools run --rm tools go run -tags=postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 $(MIGRATE_ARGS) up

migrate-down:
	docker compose --profile tools run --rm tools go run -tags=postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 $(MIGRATE_ARGS) down

migrate-force:
	docker compose --profile tools run --rm tools go run -tags=postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 $(MIGRATE_ARGS) force $(filter-out $@,$(MAKECMDGOALS))

seed-db:
	@echo ">> use root:  make seed"
	docker compose run --rm --no-deps --entrypoint /app/seed api

swag:
	docker compose --profile tools run --rm tools sh -c 'cd /src && go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go --parseDependency --output docs'

test:
	docker compose --profile tools run --rm tools go test ./... -v

test-coverage:
	docker compose --profile tools run --rm tools go test ./... -cover

# catch-all rule for composed rules such as migrate-force and new-migration
%:
	@:
