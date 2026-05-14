# PaladinAI v2 — Claude Code Instructions

## Project Overview

PaladinAI v2 is an AI-powered, multi-tenant infrastructure monitoring and incident response platform. It ingests alerts, deduplicates and correlates them in real time, then drives LLM-powered triage agents (Eino ReAct) over NATS JetStream. This is a Go 1.26 monorepo at `github.com/paladinai/paladinai`, currently on branch `v2`.

## Directory Layout

```
cmd/paladin/          CLI + Bubble Tea TUI (Cobra v1.9.1)
services/             Go microservices (one binary each under cmd/)
  paladin-ingest/     Alert ingest HTTP API, dedup, normalize, NATS publish (:9001)
  paladin-edge/       API gateway, rate limiting, routing (:9002)
  paladin-agent/      Eino ReAct triage agent, NATS consumer, OpenRouter LLM
  paladin-hub/        MCP server registry: register/list/deregister/heartbeat (:8082)
  paladin-{auth,memory,ws,orchestrator}/  (scaffolded, in progress)
internal/             Shared packages — not importable outside the module
  alert/              AlertEnvelope, Fingerprint, ValidateTenantID
  correlation/        Label-based correlator with Valkey 5-min sliding windows
  pipeline/           Deduplicator -> Correlator -> Publisher pipeline
  nats/               JetStream stream setup helpers
  telemetry/          OpenTelemetry tracing
  logger/             zap logger setup
  config/             envconfig-style base config loader
  llm/, middleware/   LLM client + HTTP middleware
gen/go/               gRPC stubs (hand-crafted shims; regenerate with `buf generate`)
proto/                Proto3 service definitions (agent, alert, incident)
docs/plans/           Stage-by-stage architecture plans (01..25)
migrations/           SQL migrations (golang-migrate)
infra/, ui/           Infra manifests and frontend (separate)
```

## Key Commands

Always use `make` — environment flags (GOPROXY, GOCACHE, GOMODCACHE) are pre-set.

```
make up               # Start NATS, Postgres, Valkey, Qdrant, FalkorDB, Vault, OTel, Prom, Grafana
make up-all           # + Hatchet
make down             # Stop infra (data preserved); make down-clean wipes volumes
make build            # Build all services to bin/
make test             # Run all unit tests (60s timeout)
make test-coverage    # Coverage report -> coverage.html
make lint             # golangci-lint
make fmt              # gofmt + goimports
make vet              # go vet
```

Per-service: `make build-ingest`, `make test-ingest`. For ad-hoc Go commands, prefer wrapping with the Makefile env or run inside `direnv`/`devbox` — raw `go test ./...` may hit TLS issues on macOS without the GOPROXY workaround.

## Build Quirks

- macOS has a TLS cert issue with the default Go module cache; the Makefile sets `GOPROXY=file://$HOME/go/pkg/mod/cache/download,http://proxy.golang.org,direct` and `GOCACHE`/`GOMODCACHE` to `$TMPDIR`. Use `make` targets and you will not hit it.
- `GOFLAGS=-mod=mod`, `GONOSUMDB=*`, `GOINSECURE=*` are exported by the Makefile.
- Proto stubs under `gen/go/` are hand-crafted shims today; if regenerating, use `buf generate` against `buf.gen.yaml`.

## Code Conventions

- **Interface-first design.** Every external dependency (NATS, Valkey, LLM, HTTP client) sits behind an interface defined in the consuming package. Concrete adapters live alongside but are wired only in `cmd/`.
- **In-memory fakes for tests.** Unit tests use hand-written fakes implementing the same interfaces. `miniredis` is acceptable for Valkey; do not start real NATS/Valkey/LLM in unit tests.
- **No external deps in unit tests.** No network, no docker, no `testcontainers` in `_test.go` files marked as unit tests. Integration tests go in separate `*_integration_test.go` files with build tags.
- **Logging:** `zap` via `internal/logger`. Structured fields only; never `fmt.Println` in production paths.
- **Errors:** wrap with `fmt.Errorf("...: %w", err)`; surface context, preserve `cause`.
- **Naming:** follow `~/.claude/rules/naming.md` Go conventions (PascalCase exported, camelCase unexported, `ID`/`URL`/`HTTP` acronym casing).
- **Coverage target:** 80%+ on `internal/` and `services/*/internal/`.

## Branch / PR Workflow

1. Cut a feature branch from `v2` (e.g. `feat/correlator-jitter`, `fix/dedup-race`).
2. Implement with TDD where practical; keep commits atomic (`feat:`, `fix:`, `refactor:`, ...).
3. Run `make fmt lint vet test` before pushing.
4. Open PR targeting `v2`. The **go-reviewer** agent must review before merge.
5. Address blockers, squash if needed, merge. Do not push directly to `v2` or `main`.

## Key Design Decisions

- **AlertEnvelope** (`internal/alert`) is the canonical wire model across services. It carries tenant ID, labels, annotations, timestamps, and a deterministic `Fingerprint`.
- **Fingerprint = SHA-256 over sorted `key=value` label pairs** (joined with a stable separator). Identical labels across senders collapse to the same fingerprint — the basis for dedup and correlation.
- **Correlation windows:** 5-minute sliding windows keyed by label subsets, stored in Valkey with TTL. Correlator emits group IDs when windows overlap on configured label keys.
- **Pipeline order:** Deduplicator -> Correlator -> Publisher (NATS JetStream). Each stage is an interface; pipeline is composed in `cmd/`.
- **NATS streams:** `PALADIN_ALERTS` (ingest -> agent), `PALADIN_AGENT_WORK` (agent task fanout). JetStream durables per consumer.
- **LLM tiering (OpenRouter):** TierA=qwen3.6-flash (cheap classification), TierB=qwen3-8b (triage), TierC=deepseek-v3.2 (deep reasoning). Agents pick tier per step.
- **Multi-tenancy:** `ValidateTenantID` is the boundary check. Every envelope must carry a valid tenant; cross-tenant access is a hard error, not a 403 — it should be impossible by construction.

## Pointers

- Architecture stages: `docs/plans/01..25.*.md`
- Decision math & thresholds: `docs/plans/03.5.decision-math-stage3.5.md`
- Agent runtime (Eino): `docs/plans/03.agent-runtime-stage3.md`
