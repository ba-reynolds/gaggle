.PHONY: dev-services dev-services-stop dev-services-status dev-backend dev-frontend migrate-up seed test test-backend test-frontend build-backend build-frontend swag services services-stop

# Backend

migrate-up:
	$(MAKE) -C server migrate-up

seed:
	$(MAKE) -C server seed-db

dev-backend:
	$(MAKE) -C server run

swag:
	$(MAKE) -C server swag

test-backend:
	$(MAKE) -C server test

build-backend:
	cd server && go build ./...

# Frontend

dev-frontend:
	cd web && npm run dev

build-frontend:
	cd web && npm run build

lint-frontend:
	cd web && npm run lint

test-frontend:
	cd web && npm run build

# Everything

test: test-backend build-frontend lint-frontend

build: build-backend build-frontend

# Local services without Docker (Nix)

dev-services:
	$(MAKE) -C server dev-services

dev-services-stop:
	$(MAKE) -C server dev-services-stop

dev-services-status:
	$(MAKE) -C server dev-services-status

# Convenience aliases
services: dev-services

services-stop: dev-services-stop
