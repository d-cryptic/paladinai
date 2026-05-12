/**
 * Auth + tenant management tests.
 * Tests the full tenant CRUD flow via paladin-edge → paladin-auth.
 */
import { test, expect } from "@playwright/test";

const ADMIN_SECRET = process.env.ADMIN_SECRET || "paladin-admin-secret";

function uniqueSlug(): string {
  return `test-tenant-${Date.now()}`;
}

test.describe("Tenant management via paladin-edge", () => {
  test("create tenant returns 201 with tenant object", async ({ request }) => {
    const slug = uniqueSlug();
    const resp = await request.post("/api/v1/auth/tenants", {
      data: { slug, name: "Test Tenant" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.data).toBeDefined();
    expect(body.data.slug).toBe(slug);
    expect(body.data.state).toBe("active");
    expect(body.data.id).toBeTruthy();
  });

  test("create duplicate tenant returns 409", async ({ request }) => {
    const slug = uniqueSlug();
    // First creation should succeed.
    const first = await request.post("/api/v1/auth/tenants", {
      data: { slug, name: "Original" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(first.status()).toBe(201);

    // Second creation of same slug should conflict.
    const second = await request.post("/api/v1/auth/tenants", {
      data: { slug, name: "Duplicate" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(second.status()).toBe(409);
  });

  test("create tenant without admin secret returns 401", async ({
    request,
  }) => {
    const resp = await request.post("/api/v1/auth/tenants", {
      data: { slug: uniqueSlug(), name: "Unauthorized" },
    });
    expect([401, 403]).toContain(resp.status());
  });

  test("list tenants returns array", async ({ request }) => {
    // Create at least one to ensure list is non-empty.
    await request.post("/api/v1/auth/tenants", {
      data: { slug: uniqueSlug(), name: "List Test" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });

    const resp = await request.get("/api/v1/auth/tenants", {
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(Array.isArray(body.data)).toBe(true);
    expect(body.data.length).toBeGreaterThan(0);
  });

  test("get tenant by id", async ({ request }) => {
    const slug = uniqueSlug();
    const created = await request.post("/api/v1/auth/tenants", {
      data: { slug, name: "Get Test" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    const { data } = await created.json();
    const id = data.id;

    const resp = await request.get(`/api/v1/auth/tenants/${id}`, {
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.data.id).toBe(id);
    expect(body.data.slug).toBe(slug);
  });

  test("issue JWT token for valid tenant", async ({ request }) => {
    const slug = uniqueSlug();
    await request.post("/api/v1/auth/tenants", {
      data: { slug, name: "Token Test" },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });

    const tokenResp = await request.post("/api/v1/auth/tokens", {
      data: { tenant_slug: slug },
      headers: { "X-Admin-Secret": ADMIN_SECRET },
    });
    expect(tokenResp.status()).toBe(201);
    const body = await tokenResp.json();
    expect(body.token).toBeTruthy();
    // JWT has 3 dot-separated parts.
    expect(body.token.split(".").length).toBe(3);
  });
});
