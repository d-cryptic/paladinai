/**
 * Runbook import, list, and search tests.
 * Tests the paladin-hub runbook API via paladin-edge (JWT-protected).
 */
import { test, expect, request } from "@playwright/test";
import { createServer, Server } from "http";

const ADMIN_SECRET = process.env.ADMIN_SECRET || "paladin-admin-secret";
const EDGE_URL = process.env.PALADIN_EDGE_URL || "http://localhost:9002";
const HUB_URL = process.env.PALADIN_HUB_URL || "http://localhost:8082";
const RUNBOOK_FIXTURE_HOST =
  process.env.RUNBOOK_FIXTURE_HOST || (process.env.CI ? "127.0.0.1" : "host.docker.internal");
const RUNBOOK_FILE_PATH =
  process.env.RUNBOOK_FILE_PATH ||
  (process.env.CI ? "test/fixtures/runbooks/db-failover.md" : "/fixtures/runbooks/db-failover.md");

let tenantSlug: string;
let tenantID: string;
let jwtToken: string;
let runbookServer: Server;
let remoteRunbookURL: string;

test.beforeAll(async () => {
  runbookServer = createServer((_req, res) => {
    res.writeHead(200, { "Content-Type": "text/markdown" });
    res.end(`# Remote Runbook

## Overview
This fixture verifies HTTP-backed runbook imports without depending on an external service.

## Steps
- Confirm the incident scope.
- Check metrics and recent deploys.
- Escalate if impact is customer-facing.
`);
  });
  await new Promise<void>((resolve) => {
    runbookServer.listen(0, "0.0.0.0", resolve);
  });
  const address = runbookServer.address();
  if (!address || typeof address === "string") {
    throw new Error("runbook fixture server did not expose a TCP address");
  }
  remoteRunbookURL = `http://${RUNBOOK_FIXTURE_HOST}:${address.port}/runbook.md`;

  tenantSlug = `runbook-test-${Date.now()}`;
  const edgeCtx = await request.newContext({ baseURL: EDGE_URL });

  const created = await edgeCtx.post("/api/v1/auth/tenants", {
    data: { slug: tenantSlug, name: "Runbook Test Tenant" },
    headers: { "X-Admin-Secret": ADMIN_SECRET },
  });
  expect(created.status()).toBe(201);
  const { data: tenant } = await created.json();
  tenantID = tenant.id;

  const tokenResp = await edgeCtx.post("/api/v1/auth/tokens", {
    data: { tenant_id: tenantID, user_id: "playwright-runbook-user", roles: ["admin"] },
    headers: { "X-Admin-Secret": ADMIN_SECRET },
  });
  expect(tokenResp.status()).toBe(200);
  const body = await tokenResp.json();
  jwtToken = body.token;
  await edgeCtx.dispose();

});

test.afterAll(async () => {
  if (!runbookServer) {
    return;
  }
  await new Promise<void>((resolve, reject) => {
    runbookServer.close((err) => (err ? reject(err) : resolve()));
  });
});

test.describe("Runbook import via paladin-hub direct", () => {
  test("import from github source returns 201", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/runbooks/import", {
      data: { source: "github", repo: remoteRunbookURL },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.imported).toBe(1);
    expect(body.data).toBeDefined();
    expect(body.data.source).toBe("github");
    await ctx.dispose();
  });

  test("import from file source indexes content", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/runbooks/import", {
      data: { source: "file", path: RUNBOOK_FILE_PATH },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.data.embedded).toBe(true);
    expect(body.data.chunks).toBeGreaterThan(0);
    await ctx.dispose();
  });

  test("import without source returns 422", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/runbooks/import", {
      data: {},
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(422);
    await ctx.dispose();
  });

  test("import without tenant header returns 400", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/runbooks/import", {
      data: { source: "github", repo: "test/repo" },
    });
    expect(resp.status()).toBe(400);
    await ctx.dispose();
  });
});

test.describe("Runbook list via paladin-hub direct", () => {
  test("list returns data array", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    // Seed at least one runbook.
    await ctx.post("/api/v1/runbooks/import", {
      data: { source: "confluence", space: remoteRunbookURL },
      headers: { "X-Tenant-ID": tenantSlug },
    });

    const resp = await ctx.get("/api/v1/runbooks", {
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(Array.isArray(body.data)).toBe(true);
    await ctx.dispose();
  });

  test("list with source filter returns matching records", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    await ctx.post("/api/v1/runbooks/import", {
      data: { source: "notion", space: remoteRunbookURL },
      headers: { "X-Tenant-ID": tenantSlug },
    });

    const resp = await ctx.get("/api/v1/runbooks?source=notion", {
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    for (const rb of body.data) {
      expect(rb.source).toBe("notion");
    }
    await ctx.dispose();
  });
});

test.describe("Runbook search via paladin-hub direct", () => {
  test.beforeAll(async () => {
    // Import a file runbook so search has indexed content.
    const ctx = await request.newContext({ baseURL: HUB_URL });
    await ctx.post("/api/v1/runbooks/import", {
      data: { source: "file", path: RUNBOOK_FILE_PATH },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    await ctx.dispose();
  });

  test("search returns results array", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/runbooks/search", {
      data: { query: "database failover postgres", top_k: 3 },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(Array.isArray(body.data)).toBe(true);
    await ctx.dispose();
  });

  test("search without query returns 422", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/runbooks/search", {
      data: { query: "" },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(422);
    await ctx.dispose();
  });
});

test.describe("Runbook API via paladin-edge (JWT-protected)", () => {
  test("import via edge with JWT returns 201", async () => {
    const ctx = await request.newContext({ baseURL: EDGE_URL });
    const resp = await ctx.post("/api/v1/runbooks/import", {
      data: { source: "github", repo: remoteRunbookURL },
      headers: {
        Authorization: `Bearer ${jwtToken}`,
        "X-Tenant-ID": tenantID,
      },
    });
    expect(resp.status()).toBe(201);
    await ctx.dispose();
  });

  test("import without JWT returns 401", async () => {
    const ctx = await request.newContext({ baseURL: EDGE_URL });
    const resp = await ctx.post("/api/v1/runbooks/import", {
      data: { source: "github", repo: "owner/no-auth" },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(401);
    await ctx.dispose();
  });
});
