-include .env
export

COMPOSE_DEV = docker compose -f deploy/docker-compose.dev.yaml --env-file .env
COMPOSE_PROD = docker compose -f deploy/docker-compose.yaml --env-file .env
MIGRATE_DSN = "postgres://$(DB_USER):$(DB_PASSWORD)@localhost:$(DB_HOST_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)"
PROFILE = $(if $(WITH),--profile $(WITH),)

CYAN := $(shell tput setaf 6 2>/dev/null)
YELLOW := $(shell tput setaf 3 2>/dev/null)
GREEN := $(shell tput setaf 2 2>/dev/null)
RESET := $(shell tput sgr0 2>/dev/null)

# =============================================================================
#  Help Command
# =============================================================================
.PHONY: help
help: ## Show this beautiful help menu (default)
	@echo ""
	@echo "$(CYAN)╔══════════════════════════════════════════════════════════╗$(RESET)"
	@echo "$(CYAN)║                   Jungo App - Makefile Help              ║$(RESET)"
	@echo "$(CYAN)╚══════════════════════════════════════════════════════════╝$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "; printf "\n"} \
		/^#  .* (Commands|Testing|Migrations|Generation|Utility|Documentation|Testing).*$$/ { printf "\n$(YELLOW)%s $(RESET)\n", substr($$0, 4) } \
		/^[a-zA-Z0-9_-]+:.*?## / { printf "  $(GREEN)%-25s$(RESET) %s\n", $$1, $$2 }' \
		$(MAKEFILE_LIST)
	@echo ""
	@echo "$(CYAN)Tips:$(RESET)"
	@echo "$(CYAN) • Run 'make' or 'make help' to see this menu.$(RESET)"
	@echo "$(CYAN) • Use 'make app-dev LOG=full' for full logs in dev mode.$(RESET)"

.DEFAULT_GOAL := help

# =============================================================================
#  Setup Commands
# =============================================================================
.PHONY: app-init
app-init: ## Interactively create .env (app name, db name/user/pass, ports) for a new clone
	@bash deploy/app-init.sh

# =============================================================================
# Development
# =============================================================================
.PHONY: app-dev app-dev-bg app-dev-down app-dev-local app-logs
app-dev: ## Start dev stack (App + PostgreSQL). Add WITH=cache or WITH=full for optional services.
	$(COMPOSE_DEV) $(PROFILE) up --build

app-dev-bg: ## Same as app-dev, detached
	$(COMPOSE_DEV) $(PROFILE) up --build -d

app-dev-down: ## Stop and remove the dev stack
	$(COMPOSE_DEV) down

app-dev-local: ## Same as app-dev, but uses local ../junkit (see go.work.local) — personal only, gitignored
	$(COMPOSE_DEV) -f deploy/docker-compose.local.yaml $(PROFILE) up --build

app-logs: ## Follow the app container's logs
	$(COMPOSE_DEV) logs -f app

# =============================================================================
# Production
# =============================================================================
.PHONY: app-prod app-prod-bg app-prod-down
app-prod: ## Build and start the prod stack (App + PostgreSQL). Add WITH=cache or WITH=full for optional services.
	$(COMPOSE_PROD) $(PROFILE) up --build

app-prod-bg: ## Same as app-prod, detached
	$(COMPOSE_PROD) $(PROFILE) up --build -d

app-prod-down: ## Stop and remove the prod stack
	$(COMPOSE_PROD) down

# =============================================================================
#  Database Migrations (require DB running - run 'make app-dev-bg' first)
#  Install: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
#  Website: https://github.com/golang-migrate/migrate/tree/master/cmd/migrate
# =============================================================================
.PHONY: migrate-create migrate-up migrate-down migrate-refresh sqlc
migrate-create: ## Create a new migration: make migrate-create NAME=add_x_table
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=<name>" && exit 1; fi
	migrate create -ext sql -dir internal/database/migrations -seq $(NAME)

migrate-up: ## Apply all pending migrations
	migrate -database $(MIGRATE_DSN) -path internal/database/migrations up

migrate-down: ## Roll back the last migration
	migrate -database $(MIGRATE_DSN) -path internal/database/migrations down 1

migrate-refresh: ## Drop everything and re-run all migrations (destroys data)
	migrate -database $(MIGRATE_DSN) -path internal/database/migrations drop -f
	$(MAKE) migrate-up

# =============================================================================
#  SQLC Generation
#  SQLC: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
#  Website: https://docs.sqlc.dev/
# =============================================================================
sqlc: ## Regenerate internal/database/sqlc from internal/database/queries
	sqlc generate

# =============================================================================
# Scaffold
# =============================================================================
.PHONY: feature feature-remove command
feature: ## Generate a new feature: make feature NAME=product [TABLE=custom_table]
	@if [ -z "$(NAME)" ]; then echo "Usage: make feature NAME=<snake_case_name> [TABLE=<table_name>]" && exit 1; fi
	go run ./cmd/scaffold -name $(NAME) -table "$(TABLE)"

feature-remove: ## Remove a generated feature: make feature-remove NAME=product
	@if [ -z "$(NAME)" ]; then echo "Usage: make feature-remove NAME=<snake_case_name>" && exit 1; fi
	go run ./cmd/scaffold -name $(NAME) -mode remove

command: ## Generate a new CLI command: make command NAME=health-check SIGNATURE=health:check [FEATURE=user]
	@if [ -z "$(NAME)" ] || [ -z "$(SIGNATURE)" ]; then echo "Usage: make command NAME=<snake_case_name> SIGNATURE=<cli:signature> [FEATURE=<feature_name>]" && exit 1; fi
	go run ./cmd/scaffold -type command -name $(NAME) -signature $(SIGNATURE) -feature "$(FEATURE)"

# =============================================================================
# Console
# =============================================================================
.PHONY: console
console: ## Run a CLI command inside whichever app stack is running, dev or prod: make console CMD="user:list" [ARGS="..."]
	@if [ -z "$(CMD)" ]; then echo "Usage: make console CMD=<command> [ARGS=...]" && exit 1; fi
	@if [ -n "$$($(COMPOSE_DEV) ps -q app 2>/dev/null)" ]; then \
		$(COMPOSE_DEV) exec app go run ./cmd/console $(CMD) $(ARGS); \
	elif [ -n "$$($(COMPOSE_PROD) ps -q app 2>/dev/null)" ]; then \
		$(COMPOSE_PROD) exec app ./console $(CMD) $(ARGS); \
	else \
		echo "No running app container found — start one first with 'make app-dev-bg' or 'make app-prod-bg'" && exit 1; \
	fi

# =============================================================================
# Go
# =============================================================================
.PHONY: build test vet fmt tidy
build: ## Compile the API binary
	go build -o bin/api ./cmd/api

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go source
	gofmt -w .

tidy: ## Tidy go.mod/go.sum
	go mod tidy
