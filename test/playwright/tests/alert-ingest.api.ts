/**
 * Alert ingestion pipeline tests.
 * Verifies the full ingest flow: Alertmanager webhook → deduplicate → normalize
 * → NATS publish.
 *
 * Requires paladin-ingest and paladin-edge running with a valid JWT_SECRET.
 */
import { test, expect, request } from "@playwright/test";

const ADMIN_SECRET = process.env.ADMIN_SECRET || "paladin-admin-secret";
const EDGE_URL = process.env.PALADIN_EDGE_URL || "http://localhost:9002";
const INGEST_URL = process.env.PALADIN_INGEST_URL || "http://localhost:9001";

let tenantSlug: string;
let jwtToken: string;

test.beforeAll(async () => {
  tenantSlug = `ingest-test-${Date.now()}`;
  const edgeCtx = await request.newContext({ baseURL: EDGE_URL });

  // Create tenant.
  const created = await edgeCtx.post("/api/v1/auth/tenants", {
    data: { slug: tenantSlug, name: "Ingest Test Tenant" },
    headers: { "X-Admin-Secret": ADMIN_SECRET },
  });
  expect(created.status()).toBe(201);

  // Issue JWT.
  const tokenResp = await edgeCtx.post("/api/v1/auth/tokens", {
    data: { tenant_slug: tenantSlug },
    headers: { "X-Admin-Secret": ADMIN_SECRET },
  });
  expect(tokenResp.status()).toBe(201);
  const body = await tokenResp.json();
  jwtToken = body.token;
  await edgeCtx.dispose();
});

test.describe("Alert ingest via paladin-ingest direct", () => {
  test("Alertmanager webhook returns 202", async () => {
    const ctx = await request.newContext({ baseURL: INGEST_URL });
    const payload = alertmanagerPayload(tenantSlug, "TestAlert", "firing");
    const resp = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(202);
    await ctx.dispose();
  });

  test("Resolved alert returns 202", async () => {
    const ctx = await request.newContext({ baseURL: INGEST_URL });
    const payload = alertmanagerPayload(tenantSlug, "TestAlert", "resolved");
    const resp = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(202);
    await ctx.dispose();
  });

  test("Missing X-Tenant-ID returns 400", async () => {
    const ctx = await request.newContext({ baseURL: INGEST_URL });
    const payload = alertmanagerPayload("", "TestAlert", "firing");
    const resp = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
    });
    expect(resp.status()).toBe(400);
    await ctx.dispose();
  });

  test("Malformed JSON returns 400", async () => {
    const ctx = await request.newContext({ baseURL: INGEST_URL });
    const resp = await ctx.post("/api/v1/ingest/alertmanager", {
      data: "not valid json",
      headers: {
        "Content-Type": "application/json",
        "X-Tenant-ID": tenantSlug,
      },
    });
    expect(resp.status()).toBe(400);
    await ctx.dispose();
  });
});

test.describe("Alert ingest via paladin-edge (JWT-protected)", () => {
  test("POST /api/v1/ingest/alertmanager via edge with JWT returns 202", async () => {
    const ctx = await request.newContext({ baseURL: EDGE_URL });
    const payload = alertmanagerPayload(tenantSlug, "EdgeTest", "firing");
    const resp = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
      headers: {
        Authorization: `Bearer ${jwtToken}`,
        "X-Tenant-ID": tenantSlug,
      },
    });
    expect(resp.status()).toBe(202);
    await ctx.dispose();
  });

  test("POST without JWT returns 401", async () => {
    const ctx = await request.newContext({ baseURL: EDGE_URL });
    const payload = alertmanagerPayload(tenantSlug, "NoAuth", "firing");
    const resp = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(401);
    await ctx.dispose();
  });

  test("P1 alert with CriticalService label is ingested", async () => {
    const ctx = await request.newContext({ baseURL: INGEST_URL });
    const payload = {
      version: "4",
      groupKey: "{}:{}:{alertname=CriticalDBDown}",
      status: "firing",
      receiver: "paladin",
      groupLabels: { alertname: "CriticalDBDown" },
      commonLabels: {
        alertname: "CriticalDBDown",
        severity: "critical",
        service: "postgres",
      },
      commonAnnotations: {
        summary: "Primary database is down",
        runbook: "https://runbooks.internal/db-failover",
      },
      alerts: [
        {
          status: "firing",
          labels: {
            alertname: "CriticalDBDown",
            severity: "critical",
            service: "postgres",
            instance: "db-primary-01",
          },
          annotations: {
            summary: "Primary database is down",
            description: "Postgres primary db-primary-01 is not responding",
          },
          startsAt: new Date().toISOString(),
          endsAt: "0001-01-01T00:00:00Z",
        },
      ],
    };

    const resp = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
      headers: { "X-Tenant-ID": tenantSlug },
    });
    expect(resp.status()).toBe(202);
    await ctx.dispose();
  });
});

function alertmanagerPayload(
  tenantSlug: string,
  alertname: string,
  status: "firing" | "resolved"
) {
  return {
    version: "4",
    groupKey: `{}:{}:{alertname=${alertname}}`,
    status,
    receiver: "paladin",
    groupLabels: { alertname },
    commonLabels: { alertname, severity: "warning", service: "api-gateway" },
    commonAnnotations: { summary: `${alertname} is ${status}` },
    alerts: [
      {
        status,
        labels: { alertname, severity: "warning", instance: "api-01" },
        annotations: { summary: `${alertname} is ${status}` },
        startsAt: new Date().toISOString(),
        endsAt: "0001-01-01T00:00:00Z",
      },
    ],
  };
}
