# PaladinAI Dashboard

This is the local React dashboard surface for v2. It uses Vite, Tailwind, lucide icons, Radix primitives, and shadcn-style local components. The layout mirrors the Stage 21 information architecture with a Linear-like operations workspace: dense navigation, incident table, selected-incident detail, metrics, trends, runbooks, and integration health.

The dashboard can run in two modes:

- Mock fallback: used when no auth token is configured or when the backend is unavailable. This keeps design and interaction tests local without consuming GitHub Actions minutes or requiring the full service stack.
- Live backend: used when `VITE_PALADIN_AUTH_TOKEN` or `localStorage.paladin-auth-token` is present. The dashboard calls paladin-edge for incidents and runbooks, then falls back to mock data if those calls fail.

Runtime configuration:

```bash
VITE_PALADIN_API_URL=http://127.0.0.1:9002/api/v1
VITE_PALADIN_WS_URL=ws://127.0.0.1:9007/v2/ws/alerts
VITE_PALADIN_TENANT_ID=acme-prod
VITE_PALADIN_AUTH_TOKEN=
```

`VITE_PALADIN_API_URL` should point at `paladin-edge`. The current live reads use `GET /incidents` and `GET /runbooks` with `Authorization: Bearer <token>` and `X-Tenant-ID: <tenant>`.

`VITE_PALADIN_WS_URL` should point at `paladin-ws`. Browser-native websocket clients cannot set `Authorization`, so the dashboard sends the token in the non-echoed `paladinai.jwt.<token>` websocket subprotocol alongside the public `paladinai.v2` protocol. The server only echoes `paladinai.v2`.

In live mode, dashboard charts and cards are derived from backend data instead of demo constants:

- Incidents, severity split, trend, SLO budget, activity, eval confidence, model tier mix, and compute load are derived from `GET /incidents`.
- Runbook index cards and semantic index counts are derived from `GET /runbooks`.
- Integration health and green-check counts are derived from `GET /mcp/servers`.
- Websocket alert frames are merged into the same dashboard model so live stream events update charts and counts.

Validation:

```bash
make playwright-ui
```

The production Next.js dashboard described in `docs/plans/21.frontend-stage21.md` should replace or extend this local Vite surface when the API contract and deployment target are finalized.
