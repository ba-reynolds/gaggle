.PHONY: dev-backend dev-frontend migrate-up seed test test-backend test-frontend build-backend build-frontend swag services services-stop

# Backend

migrate-up:
	$(MAKE) -C social-back migrate-up

seed:
	$(MAKE) -C social-back seed-db

dev-backend:
	$(MAKE) -C social-back run

swag:
	$(MAKE) -C social-back swag

test-backend:
	$(MAKE) -C social-back test

build-backend:
	cd social-back && go build ./...

# Frontend

dev-frontend:
	cd social-front && npm run dev

build-frontend:
	cd social-front && npm run build

lint-frontend:
	cd social-front && npm run lint

test-frontend:
	cd social-front && npm run build

# Everything

test: test-backend build-frontend lint-frontend

build: build-backend build-frontend

# Local services without Docker (Nix)

services:
	$(MAKE) -C social-back dev-services

services-stop:
	$(MAKE) -C social-back dev-services-stop
