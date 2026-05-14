# PaladinAI v2

AI-powered, multi-tenant infrastructure monitoring and incident response platform. PaladinAI ingests alerts from any source, deduplicates and correlates them in real time, and drives LLM-powered triage agents that investigate, hypothesize, and act.

> Active branch: **`v2`** — a full revamp for production-scale multi-tenant SaaS. See [`docs/plans/`](docs/plans/) for the staged architecture.

## Status

Phases **1 through 11** are implemented and merged to `v2`:

1. Architecture & language split
2. Messaging (NATS JetStream, dedup, correlation)
3. Agent runtime (Eino ReAct)
3.5. Decision math & thresholds
4. Workflows
4.5. Prompt engineering
5. Memory
6. RAG / hybrid search
7. Integrations (MCP hub)
8. Multitenancy hardening
9. Cache layers and cost attribution
10. Eval and local testing harness
11. Onboarding UX (`paladin init`, `doctor`, integrations, tail)

Phases 12 onward (specialised agents, gateway, observability, security, API, frontend, deployment, billing, DR, enterprise scale) are in design — see `docs/plans/12..25.*.md`.

## Services

| Service | Purpose | Default Port |
|---|---|---|
| `paladin-ingest` | Alert ingestion HTTP API; dedup, normalize, NATS publish | `9001` |
| `paladin-edge` | API gateway; rate limiting, routing, tenant auth | `9002` |
| `paladin-agent` | Eino ReAct triage/RCA agent; NATS consumer; OpenRouter LLM; incident replay API | `9006` |
| `paladin-hub` | MCP server registry (register/list/deregister/heartbeat) | `8082` |
| `paladin-auth` | Tenant registry and JWT issuance | `9003` |
| `paladin-memory` | Working/episodic/procedural/semantic memory API | `9010` HTTP, `9011` gRPC |
| `paladin-ws` | Authenticated live alert stream for CLI/UI clients | `9007` |
| `paladin-orchestrator` | Raw-alert pipeline runner and Hatchet boundary | `9008` |
| `paladin-comms` | Triage/analyzed-alert notification worker | `9009` |

CLI: `cmd/paladin` (Cobra + Bubble Tea TUI).

## Quick Start

**Prerequisites:** Go 1.26+, Docker + Docker Compose, `make`. macOS users: the Makefile handles GOPROXY/GOCACHE workarounds — use `make` targets, not raw `go` commands.

```bash
# 1. Start infrastructure (NATS, Postgres, Valkey, Qdrant, FalkorDB, Vault, OTel, Prom, Grafana)
make up

# 2. Run the unit test suite
make test

# 3. Build all service binaries into bin/
make build

# 4. Run lint + vet
make lint vet

# Teardown
make down              # stop containers, keep volumes
make down-clean        # stop and wipe data
```

See `make help` for the full target list.

## Architecture

```
                    +----------------------+
  Prometheus  --->  |                      |
  Grafana OnCall -> |   paladin-ingest     | --+
  Custom webhooks-> |  (HTTP, dedup,       |   |
                    |   normalize)         |   |
                    +----------------------+   |
                                               v
                                     +-------------------+
                                     |   NATS JetStream  |
                                     |  PALADIN_ALERTS   |
                                     +---------+---------+
                                               |
                                               v
                              +----------------+----------------+
                              |     pipeline (internal/)        |
                              |  Dedup -> Correlate -> Publish  |
                              |   (Valkey 5-min sliding wins)   |
                              +----------------+----------------+
                                               |
                                               v
                                     +-------------------+
                                     | PALADIN_AGENT_WORK|
                                     +---------+---------+
                                               |
                                               v
                    +----------------------+   |   +----------------------+
   API/UI  <------> |    paladin-edge      |<--+-->|    paladin-agent     |
                    | (gateway, rate-limit)|       | (Eino ReAct, LLM)    |
                    +----------------------+       +----------+-----------+
                                                              |
                                                              v
                                                  +-----------------------+
                                                  |     paladin-hub       |
                                                  |  (MCP tool registry)  |
                                                  +-----------------------+
```

Persistence: **Valkey 8** (dedup, correlation, rate limits), **Postgres 17** (state, audit), **Qdrant** (vector memory), **FalkorDB** (graph). Observability: **Prometheus + OpenTelemetry**. LLM routing: **OpenRouter** with three tiers (qwen3-1.7b, qwen3-8b, deepseek-v3).

## Repository Layout

```
cmd/paladin/      CLI + TUI (Cobra, Bubble Tea, lipgloss)
services/         One binary per microservice
internal/         Shared packages (alert, correlation, pipeline, nats, ...)
gen/go/           gRPC stubs (regenerate via buf)
proto/            Proto3 definitions
docs/plans/       Stage-by-stage architecture plans
migrations/       golang-migrate SQL migrations
infra/, ui/       Infra manifests and frontend
```

## Documentation

Architecture plans (staged):

- [`01.arch-stage1.md`](docs/plans/01.arch-stage1.md) — Language split, service boundaries
- [`02.messaging-stage2.md`](docs/plans/02.messaging-stage2.md) — NATS JetStream, dedup, correlation
- [`03.agent-runtime-stage3.md`](docs/plans/03.agent-runtime-stage3.md) — Eino agent graphs, tools, streaming
- [`03.5.decision-math-stage3.5.md`](docs/plans/03.5.decision-math-stage3.5.md) — Statistics, thresholds, probabilistic systems
- [`04.workflows-stage4.md`](docs/plans/04.workflows-stage4.md), [`04.5.prompt-engineering-stage4.5.md`](docs/plans/04.5.prompt-engineering-stage4.5.md)
- [`05.memory-stage5.md`](docs/plans/05.memory-stage5.md), [`06.rag-hybrid-search-stage6.md`](docs/plans/06.rag-hybrid-search-stage6.md), [`07.integrations-stage7.md`](docs/plans/07.integrations-stage7.md)
- [`08.multitenancy-stage8.md`](docs/plans/08.multitenancy-stage8.md), [`09.cache-stage9.md`](docs/plans/09.cache-stage9.md), [`10.eval-testing-stage10.md`](docs/plans/10.eval-testing-stage10.md), [`11.onboarding-ux-stage11.md`](docs/plans/11.onboarding-ux-stage11.md)
- Phases 12–25: specialised agents, gateway, observability, security, API, frontend, deployment, billing, DR, enterprise scale.

Working with Claude Code in this repo: see [`CLAUDE.md`](CLAUDE.md).

## License

Apache-2.0 with Commons Clause. See [`LICENSE`](LICENSE).
