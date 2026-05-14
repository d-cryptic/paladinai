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

`VITE_PALADIN_API_URL` should point at `paladin-edge`. The current live reads use `GET /incidents` and `GET /runbooks` with `Authorization: Bearer <token>` and `X-Tenant-ID: <tenant>`. The websocket URL is displayed in the workspace status; browser-native websocket auth still needs a server-supported cookie or subprotocol flow before it can be connected safely from the dashboard.

Validation:

```bash
make playwright-ui
```

The production Next.js dashboard described in `docs/plans/21.frontend-stage21.md` should replace or extend this local Vite surface when the API contract and deployment target are finalized.
