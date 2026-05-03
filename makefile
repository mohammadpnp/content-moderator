# Makefile
# Common commands for the project

# Variables
APP_NAME = content-moderator
BINARY_NAME = server
GO = go
GOFLAGS = -ldflags="-s -w"
DOCKER_COMPOSE = docker compose
MAIN_PATH = ./cmd/server

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: help
help: ## Show this help message
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the application binary
	@echo "${YELLOW}Building ${APP_NAME}...${RESET}"
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "${GREEN}Build complete: bin/$(BINARY_NAME)${RESET}"

.PHONY: run
run: ## Run the application locally (without Docker)
	@echo "${YELLOW}Running ${APP_NAME}...${RESET}"
	$(GO) run $(MAIN_PATH)

.PHONY: test
test: ## Run all tests
	@echo "${YELLOW}Running tests...${RESET}"
	$(GO) test -v -race -cover ./...

.PHONY: test-unit
test-unit: ## Run unit tests only
	@echo "${YELLOW}Running unit tests...${RESET}"
	$(GO) test -v -race -cover ./internal/...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "${YELLOW}Running tests with coverage...${RESET}"
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "${GREEN}Coverage report generated: coverage.html${RESET}"

.PHONY: test-coverage-func
test-coverage-func: ## Show coverage by function
	@echo "${YELLOW}Coverage by function:${RESET}"
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

.PHONY: lint
lint: ## Run golangci-lint
	@echo "${YELLOW}Running linter...${RESET}"
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code with gofmt
	@echo "${YELLOW}Formatting code...${RESET}"
	$(GO) fmt ./...
	@echo "${GREEN}Code formatted${RESET}"

.PHONY: vet
vet: ## Run go vet
	@echo "${YELLOW}Running go vet...${RESET}"
	$(GO) vet ./...

.PHONY: mod
mod: ## Tidy and verify dependencies
	@echo "${YELLOW}Tidying dependencies...${RESET}"
	$(GO) mod tidy
	$(GO) mod verify
	@echo "${GREEN}Dependencies tidy${RESET}"

.PHONY: clean
clean: ## Clean build artifacts
	@echo "${YELLOW}Cleaning...${RESET}"
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "${GREEN}Cleaned${RESET}"

# Docker commands
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "${YELLOW}Building Docker image...${RESET}"
	$(DOCKER_COMPOSE) build app

.PHONY: docker-up
docker-up: ## Start all Docker services
	@echo "${YELLOW}Starting Docker services...${RESET}"
	$(DOCKER_COMPOSE) up -d
	@echo "${GREEN}Docker services started${RESET}"
	@echo "${WHITE}Services:${RESET}"
	@echo "  App:       http://localhost:8080"
	@echo "  PostgreSQL: localhost:5432"
	@echo "  Redis:      localhost:6379"
	@echo "  NATS:       localhost:4222"
	@echo "  NATS Monitoring: http://localhost:8222"

.PHONY: docker-down
docker-down: ## Stop all Docker services
	@echo "${YELLOW}Stopping Docker services...${RESET}"
	$(DOCKER_COMPOSE) down
	@echo "${GREEN}Docker services stopped${RESET}"

.PHONY: docker-down-volumes
docker-down-volumes: ## Stop Docker services and remove volumes
	@echo "${YELLOW}Stopping Docker services and removing volumes...${RESET}"
	$(DOCKER_COMPOSE) down -v
	@echo "${GREEN}Docker services and volumes removed${RESET}"

.PHONY: docker-logs
docker-logs: ## View Docker logs
	$(DOCKER_COMPOSE) logs -f

.PHONY: docker-ps
docker-ps: ## View running Docker services
	$(DOCKER_COMPOSE) ps

.PHONY: docker-restart
docker-restart: ## Restart Docker services
	@echo "${YELLOW}Restarting Docker services...${RESET}"
	$(DOCKER_COMPOSE) restart
	@echo "${GREEN}Docker services restarted${RESET}"

# Development commands
.PHONY: dev
dev: ## Start development environment
	@echo "${YELLOW}Starting development environment...${RESET}"
	$(DOCKER_COMPOSE) up -d postgres redis nats
	@echo "${GREEN}Development services started (without app)${RESET}"
	$(GO) run $(MAIN_PATH)

.PHONY: watch
watch: ## Run with hot reload (requires air)
	@which air > /dev/null || (echo "air not installed. Run: go install github.com/air-verse/air@latest" && exit 1)
	air

# Database commands  
.PHONY: db-connect
db-connect: ## Connect to PostgreSQL
	$(DOCKER_COMPOSE) exec postgres psql -U moderator -d content_moderator

.PHONY: db-reset
db-reset: ## Reset database (WARNING: deletes all data)
	@echo "${YELLOW}Resetting database...${RESET}"
	$(DOCKER_COMPOSE) down -v postgres
	$(DOCKER_COMPOSE) up -d postgres
	@echo "${GREEN}Database reset complete${RESET}"

# Utility
.PHONY: todos
todos: ## Find TODO comments in code
	@echo "${YELLOW}TODOs in code:${RESET}"
	@grep -r "TODO" --include="*.go" . || echo "No TODOs found"

.PHONY: stats
stats: ## Show project statistics
	@echo "${YELLOW}Project Statistics:${RESET}"
	@echo "Go files: $$(find . -name '*.go' -not -path './vendor/*' | wc -l)"
	@echo "Lines of code: $$(find . -name '*.go' -not -path './vendor/*' | xargs wc -l | tail -1)"
	@echo "Test files: $$(find . -name '*_test.go' -not -path './vendor/*' | wc -l)"
	@echo "Packages: $$(find . -name '*.go' -not -path './vendor/*' -exec dirname {} \; | sort -u | wc -l)"

# Install tools
.PHONY: install-tools
install-tools: ## Install development tools
	@echo "${YELLOW}Installing development tools...${RESET}"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/air-verse/air@latest
	@echo "${GREEN}Tools installed${RESET}"

# Initialize project
.PHONY: init
init: mod fmt vet test-unit build ## Initialize project (run all checks)
	@echo "${GREEN}Project initialized successfully!${RESET}"

	# نام دیتابیس و مسیر migrationها
DB_URL = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
MIGRATIONS_PATH = deploy/migrations

.PHONY: migrate-up
migrate-up: ## اجرای همه migrationها (up)
	@echo "${YELLOW}Running migrations up...${RESET}"
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" up

.PHONY: migrate-down
migrate-down: ## بازگردانی همه migrationها (down)
	@echo "${YELLOW}Running migrations down...${RESET}"
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" down

.PHONY: migrate-drop
migrate-drop: ## حذف کامل دیتابیس (drop)
	@echo "${YELLOW}Dropping all tables...${RESET}"
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" drop

.PHONY: migrate-create
migrate-create: ## ساخت فایل migration جدید (usage: make migrate-create NAME=xxx)
	@echo "${YELLOW}Creating migration: $(NAME)...${RESET}"
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(NAME)

.PHONY: migrate-version
migrate-version: ## نمایش نسخه فعلی migration
	@echo "${YELLOW}Current migration version:${RESET}"
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" version