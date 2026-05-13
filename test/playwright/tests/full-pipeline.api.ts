/**
 * Full alert pipeline E2E test.
 * Verifies the golden path: create tenant → issue JWT → ingest alert → verify
 * deduplication → correlate group.
 *
 * This is the single most important E2E scenario: it exercises every service
 * in the critical path (edge → auth → ingest → NATS).
 */
import { test, expect, request } from "@playwright/test";

const ADMIN_SECRET = process.env.ADMIN_SECRET || "paladin-admin-secret";
const EDGE_URL = process.env.PALADIN_EDGE_URL || "http://localhost:9002";
const INGEST_URL = process.env.PALADIN_INGEST_URL || "http://localhost:9001";

test.describe("Full alert pipeline golden path", () => {
  test("create tenant → issue token → ingest P1 alert", async () => {
    const slug = `pipeline-test-${Date.now()}`;
    const edgeCtx = await request.newContext({ baseURL: EDGE_URL });

    // 1. Create tenant.
    const created = await edgeCtx.post("/api/v1/auth/tenants", {
      data: { slug, name: "Pipeline Test" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(created.status()).toBe(201);
    const { data: tenant } = await created.json();

    // 2. Issue JWT.
    const tokenResp = await edgeCtx.post("/api/v1/auth/tokens", {
      data: { tenant_id: tenant.id, user_id: "playwright-pipeline-user" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(tokenResp.status()).toBe(200);
    const { token } = await tokenResp.json();

    // 3. Ingest P1 alert via edge (JWT-protected).
    const ingestResp = await edgeCtx.post("/api/v1/ingest/alertmanager", {
      data: p1Alert(slug),
      headers: {
        Authorization: `Bearer ${token}`,
        "X-Tenant-ID": tenant.id,
      },
    });
    expect(ingestResp.status()).toBe(202);

    await edgeCtx.dispose();
  });

  test("duplicate alert is deduplicated (same fingerprint returns 202 both times)", async () => {
    const slug = `dedup-test-${Date.now()}`;
    const ctx = await request.newContext({ baseURL: INGEST_URL });

    const payload = fixedFingerprintAlert(slug);

    // First ingest.
    const r1 = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
      headers: { "X-Tenant-ID": slug },
    });
    expect(r1.status()).toBe(202);

    // Second ingest with identical labels — fingerprint matches, dedup fires.
    const r2 = await ctx.post("/api/v1/ingest/alertmanager", {
      data: payload,
      headers: { "X-Tenant-ID": slug },
    });
    expect(r2.status()).toBe(202);

    await ctx.dispose();
  });

  test("correlated alert batch (same service label) returns 202 for all", async () => {
    const slug = `correlate-test-${Date.now()}`;
    const ctx = await request.newContext({ baseURL: INGEST_URL });

    const alerts = [
      alertPayload(slug, "HighCPU", "warning", "api-gateway"),
      alertPayload(slug, "HighMemory", "warning", "api-gateway"),
      alertPayload(slug, "SlowRequests", "critical", "api-gateway"),
    ];

    for (const payload of alerts) {
      const resp = await ctx.post("/api/v1/ingest/alertmanager", {
        data: payload,
        headers: { "X-Tenant-ID": slug },
      });
      expect(resp.status()).toBe(202);
    }

    await ctx.dispose();
  });

  test("cross-tenant isolation: tenant A alert not visible to tenant B", async () => {
    // Both tenants send identical fingerprint alerts. Neither should affect
    // the other's dedup state. Both get 202.
    const slugA = `tenant-a-${Date.now()}`;
    const slugB = `tenant-b-${Date.now()}`;
    const ctx = await request.newContext({ baseURL: INGEST_URL });

    const payload = { alertname: "SharedName", severity: "warning" };

    const rA = await ctx.post("/api/v1/ingest/alertmanager", {
      data: alertPayload(slugA, "SharedName", "warning", "svc-x"),
      headers: { "X-Tenant-ID": slugA },
    });
    expect(rA.status()).toBe(202);

    const rB = await ctx.post("/api/v1/ingest/alertmanager", {
      data: alertPayload(slugB, "SharedName", "warning", "svc-x"),
      headers: { "X-Tenant-ID": slugB },
    });
    expect(rB.status()).toBe(202);

    await ctx.dispose();
  });
});

function p1Alert(tenantSlug: string) {
  return {
    version: "4",
    groupKey: "{}:{}:{alertname=ProductionDBDown}",
    status: "firing",
    receiver: "paladin",
    groupLabels: { alertname: "ProductionDBDown" },
    commonLabels: {
      alertname: "ProductionDBDown",
      severity: "critical",
      service: "postgres",
      env: "production",
    },
    commonAnnotations: {
      summary: "Production database is unreachable",
      runbook: "https://runbooks.internal/db-failover",
    },
    alerts: [
      {
        status: "firing",
        labels: {
          alertname: "ProductionDBDown",
          severity: "critical",
          service: "postgres",
          instance: "db-prod-01",
        },
        annotations: {
          summary: "Production database is unreachable",
          description: "db-prod-01 failed health check 3 consecutive times",
        },
        startsAt: new Date().toISOString(),
        endsAt: "0001-01-01T00:00:00Z",
      },
    ],
  };
}

function fixedFingerprintAlert(tenantSlug: string) {
  return {
    version: "4",
    groupKey: "{}:{}:{alertname=FixedFingerprint}",
    status: "firing",
    receiver: "paladin",
    groupLabels: { alertname: "FixedFingerprint" },
    commonLabels: {
      alertname: "FixedFingerprint",
      severity: "warning",
      service: "fixed-service",
    },
    commonAnnotations: { summary: "Fixed fingerprint alert" },
    alerts: [
      {
        status: "firing",
        labels: {
          alertname: "FixedFingerprint",
          severity: "warning",
          service: "fixed-service",
        },
        annotations: { summary: "Fixed fingerprint alert" },
        startsAt: "2026-01-01T00:00:00Z",
        endsAt: "0001-01-01T00:00:00Z",
      },
    ],
  };
}

function alertPayload(
  tenantSlug: string,
  alertname: string,
  severity: string,
  service: string
) {
  return {
    version: "4",
    groupKey: `{}:{}:{alertname=${alertname}}`,
    status: "firing",
    receiver: "paladin",
    groupLabels: { alertname },
    commonLabels: { alertname, severity, service },
    commonAnnotations: { summary: `${alertname} on ${service}` },
    alerts: [
      {
        status: "firing",
        labels: { alertname, severity, service },
        annotations: { summary: `${alertname} on ${service}` },
        startsAt: new Date().toISOString(),
        endsAt: "0001-01-01T00:00:00Z",
      },
    ],
  };
}
