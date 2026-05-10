.PHONY: help up down build test lint fmt clean dev hatchet-token

# Build env flags (common to all go commands)
GOFLAGS     := -mod=mod
GONOSUMDB   := *
GOINSECURE  := *
GOCACHE     := $(TMPDIR)/gocache
GOMODCACHE  := $(TMPDIR)/gomodcache
GOPROXY     := file://$(HOME)/go/pkg/mod/cache/download,http://proxy.golang.org,direct

export GOFLAGS GONOSUMDB GOINSECURE GOCACHE GOMODCACHE GOPROXY

SERVICES := paladin-ingest paladin-edge paladin-agent paladin-hub paladin-memory paladin-auth paladin-ws paladin-orchestrator

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Infrastructure ──────────────────────────────────────────────────────────

up: ## Start all infrastructure services (NATS, Postgres, Valkey, Qdrant, etc.)
	docker compose up -d nats postgres valkey qdrant falkordb vault otel-collector prometheus grafana
	@echo "⏳ Waiting for services..."
	@sleep 3
	@docker compose ps

up-all: ## Start full stack including Hatchet
	docker compose up -d

down: ## Stop all infrastructure services
	docker compose down

down-clean: ## Stop services and remove all data volumes
	docker compose down -v

logs: ## Tail all service logs
	docker compose logs -f

# ─── Build ───────────────────────────────────────────────────────────────────

build: ## Build all services
	@for svc in $(SERVICES); do \
		echo "→ building $$svc"; \
		go build -o bin/$$svc ./services/$$svc/cmd/ || exit 1; \
	done

build-ingest: ## Build paladin-ingest only
	go build -o bin/paladin-ingest ./services/paladin-ingest/cmd/

# ─── Test ────────────────────────────────────────────────────────────────────

test: ## Run all unit tests
	go test ./... -count=1 -timeout=60s

test-v: ## Run all tests with verbose output
	go test ./... -count=1 -timeout=60s -v

test-coverage: ## Run tests with coverage report
	go test ./... -count=1 -timeout=60s -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-ingest: ## Test paladin-ingest only
	go test ./services/paladin-ingest/... -v

# ─── Code quality ────────────────────────────────────────────────────────────

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format all Go code
	gofmt -w .
	goimports -w . 2>/dev/null || true

vet: ## Run go vet
	go vet ./...

# ─── Dev ─────────────────────────────────────────────────────────────────────

dev-ingest: ## Run paladin-ingest with hot reload (air)
	air -c .air/ingest.toml

# ─── Database ────────────────────────────────────────────────────────────────

migrate-up: ## Run all pending migrations
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Rollback last migration
	migrate -path migrations -database "$(DATABASE_URL)" down 1

# ─── Hatchet ─────────────────────────────────────────────────────────────────

hatchet-token: ## Generate Hatchet API token (run after make up-all)
	@docker compose exec hatchet-api sh -c \
		"hatchet-admin token create --name=paladin-dev --tenant-id=00000000-0000-0000-0000-000000000000" \
		2>/dev/null || echo "Hatchet not running — run 'make up-all' first"

# ─── Clean ───────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html
