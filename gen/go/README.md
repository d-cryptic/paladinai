# gen/go — gRPC generated stubs

This directory contains Go stubs for the PaladinAI gRPC services.

## Current state: hand-maintained compile-only shims

The `.go` files here are **hand-maintained shims**, not real protoc output.
They exist so the monorepo compiles in CI without requiring the `buf` toolchain.

**Limitations:**
- Types do NOT implement `proto.Message` — real gRPC wire calls will fail.
- No `RegisterXxxServer` / `ServiceDesc` — servers cannot be registered with `grpc.NewServer()`.
- Timestamp fields use `*timestamppb.Timestamp` (correct type), but without `ProtoReflect`, `protojson` will not marshal them correctly.

## Regenerating proper stubs

Install buf and run:

```sh
buf generate
```

This reads `buf.gen.yaml` at the repo root and writes real protoc-gen-go output to this directory, replacing the shims.

## Directory layout

```
gen/go/
  alert/v1/alert.go       — AlertService (IngestAlert, ListAlerts, GetAlert)
  incident/v1/incident.go — IncidentService (CreateIncident, GetIncident, UpdateIncident, ListIncidents)
  agent/v1/agent.go       — AgentService (TriageAlert, GetAgentStatus)
```
