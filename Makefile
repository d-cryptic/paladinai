.PHONY: help up down build test lint fmt clean dev hatchet-token e2e integration-test playwright playwright-install eval-golden eval-live

# Build env flags (common to all go commands)
TMPDIR      ?= /tmp
GOFLAGS     := -mod=mod
GONOSUMDB   := *
GOINSECURE  := *
GOCACHE     := $(TMPDIR)/gocache
GOMODCACHE  := $(TMPDIR)/gomodcache
GOPROXY     := file://$(HOME)/go/pkg/mod/cache/download,http://proxy.golang.org,direct

export TMPDIR GOFLAGS GONOSUMDB GOINSECURE GOCACHE GOMODCACHE GOPROXY

SERVICES := paladin-ingest paladin-edge paladin-agent paladin-hub paladin-memory paladin-auth paladin-ws paladin-orchestrator paladin-comms

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Infrastructure ──────────────────────────────────────────────────────────

up: ## Start all infrastructure services (NATS, Postgres, Valkey, Qdrant, etc.)
	docker compose up -d nats postgres valkey qdrant falkordb vault otel-collector prometheus grafana
	@echo "Waiting for services..."
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

# ─── Docker images for PaladinAI services ───────────────────────────────────

build-images: ## Build Docker images for all PaladinAI services
	docker compose build $(SERVICES)

up-services: up migrate-up ## Start infra + all PaladinAI services
	docker compose up -d $(SERVICES)

logs-services: ## Follow logs from all PaladinAI services
	docker compose logs -f $(SERVICES)

# ─── Build ───────────────────────────────────────────────────────────────────

build: ## Build all services
	@for svc in $(SERVICES); do \
		echo "→ building $$svc"; \
		go build -o bin/$$svc ./services/$$svc/cmd/ || exit 1; \
	done

build-ingest: ## Build paladin-ingest only
	go build -o bin/paladin-ingest ./services/paladin-ingest/cmd/

build-comms: ## Build paladin-comms only
	go build -o bin/paladin-comms ./services/paladin-comms/cmd/main.go

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
	docker compose run --rm migrate

migrate-down: ## Rollback last migration
	docker compose run --rm migrate -path=/migrations -database="postgres://paladin:$${PG_PASSWORD:-paladin}@postgres:5432/paladin?sslmode=disable" down 1

# ─── Hatchet ─────────────────────────────────────────────────────────────────

hatchet-token: ## Generate Hatchet API token (run after make up-all)
	@docker compose exec hatchet-api sh -c \
		"hatchet-admin token create --name=paladin-dev --tenant-id=00000000-0000-0000-0000-000000000000" \
		2>/dev/null || echo "Hatchet not running — run 'make up-all' first"

# ─── Clean ───────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html

eval-smoke: ## Run smoke eval suite (no LLM, CI mode)
	go run ./cmd/paladin-eval/... --fixtures test/fixtures/ --threshold 0.8

eval-golden: ## Regenerate and run 1000-case golden eval suite with JSON metrics
	go run test/fixtures/generate_golden_pipeline_workflow.go
	go run ./cmd/paladin-eval/... --fixtures test/fixtures/ --threshold 0.8 --json

eval-live: ## Run opt-in live OpenRouter eval (uses .env OPENROUTER_API_KEY)
	go run ./services/paladin-agent/cmd/live-eval/... --fixtures test/fixtures/ --max $${LIVE_EVAL_MAX:-20} --model "$${LIVE_EVAL_MODEL:-$${LLM_TIER_A:-qwen/qwen3.6-flash}}" --shuffle --seed $${LIVE_EVAL_SEED:-0} --retries $${LIVE_EVAL_RETRIES:-4} --case-delay $${LIVE_EVAL_CASE_DELAY:-2s} --json

e2e: ## Run E2E smoke tests (requires all services running via make up)
	go test -tags e2e -timeout 120s ./test/e2e/...

integration-test: ## Run integration tests (requires NATS, Valkey, Postgres)
	go test -tags integration -timeout 120s ./...

playwright-install: ## Install Playwright and its browser dependencies
	cd test/playwright && bun install && bunx playwright install --with-deps chromium

playwright: ## Run Playwright API tests (requires all services running via make up)
	cd test/playwright && bun run test:api

playwright-ci: ## Run Playwright in CI mode (headless, GitHub reporter)
	cd test/playwright && bun run test:ci
