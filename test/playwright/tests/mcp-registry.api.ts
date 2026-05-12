/**
 * MCP server registry tests.
 * Tests register, list, heartbeat, and deregister via paladin-hub directly
 * and via the paladin-edge proxy (JWT-protected).
 */
import { test, expect, request } from "@playwright/test";

const ADMIN_SECRET = process.env.ADMIN_SECRET || "paladin-admin-secret";
const EDGE_URL = process.env.PALADIN_EDGE_URL || "http://localhost:9002";
const HUB_URL = process.env.PALADIN_HUB_URL || "http://localhost:8082";

let tenantSlug: string;
let jwtToken: string;

test.beforeAll(async () => {
  tenantSlug = `mcp-test-${Date.now()}`;
  const edgeCtx = await request.newContext({ baseURL: EDGE_URL });

  const created = await edgeCtx.post("/api/v1/auth/tenants", {
    data: { slug: tenantSlug, name: "MCP Test Tenant" },
    headers: { "X-Admin-Secret": ADMIN_SECRET },
  });
  expect(created.status()).toBe(201);

  const tokenResp = await edgeCtx.post("/api/v1/auth/tokens", {
    data: { tenant_slug: tenantSlug },
    headers: { "X-Admin-Secret": ADMIN_SECRET },
  });
  const body = await tokenResp.json();
  jwtToken = body.token;
  await edgeCtx.dispose();
});

test.describe("MCP registry via paladin-hub direct", () => {
  test("register MCP server returns 201", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/mcp/servers", {
      data: {
        endpoint: "http://kubernetes-mcp:9010",
        name: "kubernetes-mcp",
        tools: ["kubectl_get", "kubectl_apply", "kubectl_logs"],
        version: "1.0.0",
      },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.data.id).toBeTruthy();
    expect(body.data.endpoint).toBe("http://kubernetes-mcp:9010");
    await ctx.dispose();
  });

  test("list MCP servers returns array", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.get("/api/v1/mcp/servers", {
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(Array.isArray(body.data)).toBe(true);
    await ctx.dispose();
  });

  test("heartbeat for existing server returns 200", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });

    // Register first.
    const registered = await ctx.post("/api/v1/mcp/servers", {
      data: {
        endpoint: "http://prom-mcp:9011",
        name: "prometheus-mcp",
        tools: ["query_range", "instant_query"],
        version: "1.0.0",
      },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    const { data } = await registered.json();
    const serverID = data.id;

    // Heartbeat.
    const hbResp = await ctx.post(
      `/api/v1/mcp/servers/${serverID}/heartbeat`,
      {
        headers: { "X-Tenant-ID": tenantSlug },
      }
    );
    expect(hbResp.status()).toBe(200);
    await ctx.dispose();
  });

  test("deregister server returns 204", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });

    const registered = await ctx.post("/api/v1/mcp/servers", {
      data: {
        endpoint: "http://ephemeral-mcp:9099",
        name: "ephemeral-mcp",
        tools: ["tool_a"],
        version: "0.1.0",
      },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    const { data } = await registered.json();

    const delResp = await ctx.delete(
      `/api/v1/mcp/servers/${data.id}`,
      {
        headers: { "X-Tenant-ID": tenantSlug },
      }
    );
    expect(delResp.status()).toBe(204);
    await ctx.dispose();
  });

  test("register with invalid endpoint returns 422", async () => {
    const ctx = await request.newContext({ baseURL: HUB_URL });
    const resp = await ctx.post("/api/v1/mcp/servers", {
      data: {
        endpoint: "not-a-url",
        name: "bad-server",
        tools: [],
        version: "1.0.0",
      },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(422);
    await ctx.dispose();
  });
});

test.describe("MCP registry via paladin-edge (JWT-protected)", () => {
  test("register MCP server via edge with JWT returns 201", async () => {
    const ctx = await request.newContext({ baseURL: EDGE_URL });
    const resp = await ctx.post("/api/v1/mcp/servers", {
      data: {
        endpoint: "http://edge-mcp:9012",
        name: "edge-mcp-test",
        tools: ["get_runbook", "get_metrics"],
        version: "1.0.0",
      },
      headers: {
        Authorization: `Bearer ${jwtToken}`,
        "X-Tenant-ID": tenantSlug,
      },
    });
    expect(resp.status()).toBe(201);
    await ctx.dispose();
  });

  test("register without JWT returns 401", async () => {
    const ctx = await request.newContext({ baseURL: EDGE_URL });
    const resp = await ctx.post("/api/v1/mcp/servers", {
      data: {
        endpoint: "http://no-auth-mcp:9013",
        name: "no-auth-mcp",
        tools: ["tool_x"],
        version: "1.0.0",
      },
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(401);
    await ctx.dispose();
  });
});
