# PaladinAI Dashboard

This is the local React dashboard surface for v2. It uses Vite, Tailwind, lucide icons, Radix primitives, and shadcn-style local components. The layout mirrors the Stage 21 information architecture with a Linear-like operations workspace: dense navigation, incident table, selected-incident detail, metrics, trends, runbooks, and integration health.

The current version uses local mock data so design and interaction tests can run without consuming GitHub Actions minutes or requiring the full service stack.

Validation:

```bash
make playwright-ui
```

The production Next.js dashboard described in `docs/plans/21.frontend-stage21.md` should replace or extend this local Vite surface when the API contract and deployment target are finalized.
