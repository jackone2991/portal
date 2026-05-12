# Portal — common dev tasks. Run `make help` to see targets.

SHELL := /bin/bash
COMPOSE := docker compose

.DEFAULT_GOAL := help

# ── Environment ─────────────────────────────────────────────────
.PHONY: env
env: ## Copy .env.example -> .env if missing
	@test -f .env || (cp .env.example .env && echo "Created .env — fill in secrets before running 'make up'.")

# ── Stack lifecycle ─────────────────────────────────────────────
.PHONY: up down restart logs ps
up: env ## Start all services in background
	$(COMPOSE) up -d

down: ## Stop and remove containers (keeps volumes)
	$(COMPOSE) down

restart: ## Restart all services
	$(COMPOSE) restart

logs: ## Tail logs (set svc=api to filter)
	$(COMPOSE) logs -f $(svc)

ps: ## List running services
	$(COMPOSE) ps

# ── Development ─────────────────────────────────────────────────
.PHONY: dev dev-api dev-worker dev-frontend
dev: ## Run api + worker + frontend with hot reload (requires local Go and Node)
	@echo "Starting api, worker, and frontend in parallel — Ctrl-C to stop."
	@$(MAKE) -j 3 dev-api dev-worker dev-frontend

dev-api:
	cd backend && air -c .air.api.toml || go run ./cmd/api

dev-worker:
	cd backend && air -c .air.worker.toml || go run ./cmd/worker

dev-frontend:
	cd frontend && pnpm dev

# ── Database ────────────────────────────────────────────────────
.PHONY: migrate migrate-down migrate-new sqlc
migrate: ## Apply all pending migrations
	cd backend && migrate -path db/migrations -database "$$DATABASE_URL" up

migrate-down: ## Roll back the last migration
	cd backend && migrate -path db/migrations -database "$$DATABASE_URL" down 1

migrate-new: ## Create a new migration: make migrate-new name=add_movies
	@test -n "$(name)" || (echo "usage: make migrate-new name=<snake_case>"; exit 1)
	cd backend && migrate create -ext sql -dir db/migrations -seq $(name)

sqlc: ## Generate Go from SQL (db/queries/*.sql -> internal/repository/)
	cd backend && sqlc generate

# ── API contract ────────────────────────────────────────────────
.PHONY: openapi openapi-go openapi-ts
openapi: openapi-go openapi-ts ## Regenerate Go server + TS client from shared/openapi.yaml

openapi-go:
	cd backend && oapi-codegen -config oapi-codegen.yaml ../shared/openapi.yaml > internal/handler/api.gen.go

openapi-ts:
	cd frontend && pnpm openapi-typescript ../shared/openapi.yaml -o src/lib/types.gen.ts

# ── Quality ─────────────────────────────────────────────────────
.PHONY: test test-backend test-frontend lint
test: test-backend test-frontend ## Run all tests

test-backend:
	cd backend && go test ./... -race -count=1

test-frontend:
	cd frontend && pnpm test

lint: ## Run linters
	cd backend && golangci-lint run
	cd frontend && pnpm lint

# ── Build ───────────────────────────────────────────────────────
.PHONY: build
build: ## Build production images
	$(COMPOSE) build api worker frontend

# ── Help ────────────────────────────────────────────────────────
.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
