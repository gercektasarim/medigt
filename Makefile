.PHONY: help dev server cli build test migrate-up migrate-down sqlc clean setup start stop check db-up db-down db-reset

ENV_FILE ?= .env

ifneq ($(wildcard $(ENV_FILE)),)
include $(ENV_FILE)
endif

POSTGRES_DB ?= medigt
POSTGRES_USER ?= medigt
POSTGRES_PASSWORD ?= medigt
POSTGRES_PORT ?= 5450
PORT ?= 8088
FRONTEND_PORT ?= 3008
FRONTEND_ORIGIN ?= http://localhost:$(FRONTEND_PORT)
MEDIGT_APP_URL ?= $(FRONTEND_ORIGIN)
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
NEXT_PUBLIC_API_URL ?= http://localhost:$(PORT)
NEXT_PUBLIC_WS_URL ?= ws://localhost:$(PORT)/ws
GOOGLE_REDIRECT_URI ?= $(FRONTEND_ORIGIN)/auth/callback
MEDIGT_SERVER_URL ?= ws://localhost:$(PORT)/ws

export

COMPOSE := docker compose
MEDIGT_ARGS ?= $(ARGS)

.DEFAULT_GOAL := help

##@ Help

help: ## Show available make targets
	@awk 'BEGIN {FS = ":.*## "; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nQuick start:\n  \033[36mmake dev\033[0m          Bootstrap and start everything\n  \033[36mmake check\033[0m        Full local verification pipeline\n\n"} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next} \
		/^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ One-click

dev: ## Bootstrap end-to-end: create env if needed, ensure DB, migrate, start services
	@if [ ! -f .env ]; then \
		echo "==> Creating .env from .env.example..."; \
		cp .env.example .env; \
		JWT=$$(openssl rand -hex 32); \
		if [ "$$(uname)" = "Darwin" ]; then \
			sed -i '' "s/^JWT_SECRET=.*/JWT_SECRET=$$JWT/" .env; \
		else \
			sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$$JWT/" .env; \
		fi; \
		echo "==> Generated random JWT_SECRET"; \
	fi
	@$(MAKE) setup
	@$(MAKE) start

setup: ## Install deps, ensure DB, run migrations
	@echo "==> Installing dependencies..."
	pnpm install
	@$(MAKE) db-up
	@sleep 3
	@echo "==> Running migrations..."
	cd server && go run ./cmd/migrate up
	@echo ""
	@echo "OK Setup complete. Run 'make start' to launch the app."

start: ## Start backend and frontend together
	@echo "Backend:  http://localhost:$(PORT)"
	@echo "Frontend: http://localhost:$(FRONTEND_PORT)"
	@$(MAKE) db-up
	@echo "Running migrations..."
	cd server && go run ./cmd/migrate up
	@echo "Starting backend and frontend..."
	@trap 'kill 0' EXIT; \
		(cd server && go run ./cmd/server) & \
		pnpm dev:web & \
		wait

stop: ## Stop backend and frontend processes
	@echo "Stopping services..."
	@-lsof -ti:$(PORT) | xargs kill -9 2>/dev/null || true
	@-lsof -ti:$(FRONTEND_PORT) | xargs kill -9 2>/dev/null || true
	@echo "OK App processes stopped."

check: ## Run typecheck, TS tests, Go tests, and E2E
	pnpm typecheck
	pnpm test
	cd server && go test ./...
	pnpm exec playwright test

e2e: ## Run Playwright e2e tests (backend + frontend must already be up)
	pnpm exec playwright test

e2e-ui: ## Open the Playwright UI for interactive e2e debugging
	pnpm exec playwright test --ui

e2e-install: ## Install Playwright browsers (one-time setup)
	pnpm exec playwright install chromium

##@ Individual

server: ## Run only the Go server
	@$(MAKE) db-up
	cd server && go run ./cmd/server

cli: ## Run the medigt CLI from source (use ARGS="...")
	cd server && go run ./cmd/medigt $(MEDIGT_ARGS)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

build: ## Build server, CLI, and migrate binaries into server/bin
	cd server && go build -o bin/server ./cmd/server
	cd server && go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o bin/medigt ./cmd/medigt
	cd server && go build -o bin/migrate ./cmd/migrate

test: ## Run Go tests
	@$(MAKE) db-up
	cd server && go run ./cmd/migrate up
	cd server && go test ./...

##@ Database

migrate-up: ## Apply database migrations
	@$(MAKE) db-up
	cd server && go run ./cmd/migrate up

migrate-down: ## Roll back database migrations
	cd server && go run ./cmd/migrate down

sqlc: ## Regenerate sqlc code
	cd server && sqlc generate

db-up: ## Start the local PostgreSQL container
	@$(COMPOSE) up -d postgres redis

db-down: ## Stop the local containers (preserves volumes)
	@$(COMPOSE) down

db-reset: ## Drop and recreate the database, then re-run all migrations
	@case "$(DATABASE_URL)" in \
		""|*@localhost:*|*@localhost/*|*@127.0.0.1:*|*@127.0.0.1/*) ;; \
		*) echo "Refusing to reset: DATABASE_URL points at a remote host."; exit 1 ;; \
	esac
	@$(MAKE) db-up
	@sleep 2
	@echo "==> Dropping and recreating database '$(POSTGRES_DB)'..."
	@$(COMPOSE) exec -T postgres psql -U $(POSTGRES_USER) -d postgres -v ON_ERROR_STOP=1 \
		-c "DROP DATABASE IF EXISTS \"$(POSTGRES_DB)\" WITH (FORCE);" \
		-c "CREATE DATABASE \"$(POSTGRES_DB)\";"
	@echo "==> Running migrations..."
	cd server && go run ./cmd/migrate up
	@echo ""
	@echo "OK Database '$(POSTGRES_DB)' reset."

##@ Cleanup

clean: ## Remove generated server binaries and temp files
	rm -rf server/bin server/tmp
