/**
 * Health check tests — verifies all PaladinAI services respond to liveness
 * and readiness probes. Run against a live stack: make up && make up-services.
 */
import { test, expect, request } from "@playwright/test";

const SERVICES: Record<string, string> = {
  "paladin-edge": process.env.PALADIN_EDGE_URL || "http://localhost:9002",
  "paladin-ingest": process.env.PALADIN_INGEST_URL || "http://localhost:9001",
  "paladin-auth": process.env.PALADIN_AUTH_URL || "http://localhost:9003",
  "paladin-hub": process.env.PALADIN_HUB_URL || "http://localhost:8082",
};

for (const [name, baseURL] of Object.entries(SERVICES)) {
  test(`${name} /healthz returns 200`, async () => {
    const ctx = await request.newContext({ baseURL });
    const resp = await ctx.get("/healthz");
    expect(resp.status()).toBe(200);
    await ctx.dispose();
  });

  test(`${name} /readyz returns 200`, async () => {
    const ctx = await request.newContext({ baseURL });
    const resp = await ctx.get("/readyz");
    expect(resp.status()).toBe(200);
    await ctx.dispose();
  });
}

test("paladin-edge /api/v1/ping returns pong", async ({ request }) => {
  const resp = await request.get("/api/v1/ping");
  expect(resp.status()).toBe(200);
  const body = await resp.json();
  expect(body.message).toBe("pong");
  expect(body.service).toBe("paladin-edge");
});
