# PaladinAI Dashboard

This is the dependency-light local dashboard surface for v2. It mirrors the Stage 21 information architecture with a working incident table, selected-incident detail pane, metrics, severity split, runbook status, and integration health.

Open `index.html` directly in a browser for local review. The current version uses local mock data so design and interaction tests can run without consuming GitHub Actions minutes or requiring the full service stack.

Validation:

```bash
cd test/playwright
bun run test -- --project=ui-dashboard
```

The production Next.js dashboard described in `docs/plans/21.frontend-stage21.md` should replace this static surface when the API contract and deployment target are finalized.
