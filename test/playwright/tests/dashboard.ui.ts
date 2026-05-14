import { expect, test } from "@playwright/test";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import http from "node:http";

const dashboardURL = "http://127.0.0.1:4173";
let dashboardServer: ChildProcessWithoutNullStreams;

test.beforeAll(async () => {
  dashboardServer = spawn("npm", ["run", "dev", "--", "--port", "4173"], {
    cwd: "../../ui/dashboard",
    stdio: "pipe",
  });
  await waitForDashboard();
});

test.afterAll(() => {
  dashboardServer?.kill();
});

test.describe("local dashboard", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(dashboardURL);
  });

  test("renders incident command overview", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Operations dashboard" })).toBeVisible();
    await expect(page.getByText("Active incidents")).toBeVisible();
    await expect(page.getByText("Payment DB connection exhaustion")).toBeVisible();
    await expect(page.getByText("Connection pool saturation")).toBeVisible();
    await expect(page.getByText("Severity split")).toBeVisible();
    await expect(page.getByText("Agent path")).toBeVisible();
    await expect(page.getByText("Error budget")).toBeVisible();
    await expect(page.getByText("Activity")).toBeVisible();
    await expect(page.getByText("Eval quality")).toBeVisible();
    await expect(page.getByText("Model mix")).toBeVisible();
  });

  test("filters incidents and updates selected detail", async ({ page }) => {
    await page.getByLabel("Severity filter").selectOption("P2");

    await expect(page.getByText("Auth latency spike")).toBeVisible();
    await expect(page.getByText("Payment DB connection exhaustion")).toHaveCount(0);
    await expect(page.getByRole("heading", { name: /INC-2039/ })).toBeVisible();
    await expect(page.getByTestId("detail-action")).toHaveText("Scale auth-cache read replicas");
  });

  test("supports theme toggle without layout overflow", async ({ page }) => {
    await page.getByRole("button", { name: "Dark" }).click();
    await expect(page.getByRole("button", { name: "Light" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    expect(overflow).toBe(false);
  });

  test("opens command palette and jumps to incidents", async ({ page }) => {
    await page.getByRole("button", { name: "Open command menu" }).click();

    await expect(page.getByRole("dialog", { name: "Command menu" })).toBeVisible();
    await page.getByPlaceholder("Jump to incident, runbook, integration, filter...").fill("auth latency");
    await page.getByRole("button", { name: /INC-2039/ }).click();

    await expect(page.getByRole("dialog", { name: "Command menu" })).toHaveCount(0);
    await expect(page.getByRole("heading", { name: /INC-2039/ })).toBeVisible();
    await expect(page.getByTestId("detail-action")).toHaveText("Scale auth-cache read replicas");
  });

  test("navigates dashboard pages and opens key flows", async ({ page }) => {
    await page.getByRole("button", { name: "Runbooks" }).click();
    await expect(page.getByRole("heading", { name: "Runbooks" })).toBeVisible();
    await page.getByRole("button", { name: "New runbook" }).click();
    await expect(page.getByText("Create runbook")).toBeVisible();
    await page.getByRole("button", { name: "Close drawer" }).click();

    await page.getByRole("button", { name: "Integrations" }).click();
    await expect(page.getByRole("heading", { name: "Integrations" })).toBeVisible();
    await page.getByRole("button", { name: "Configure" }).first().click();
    await expect(page.getByRole("dialog", { name: "Configure integration" })).toBeVisible();
    await page.getByRole("button", { name: "Close dialog" }).click();

    await page.getByRole("button", { name: "Evals" }).click();
    await expect(page.getByRole("heading", { name: "Evals" })).toBeVisible();
    await expect(page.getByText("1000 golden cases")).toBeVisible();

    await page.getByRole("button", { name: "Agents" }).click();
    await expect(page.getByRole("heading", { name: "Agents" })).toBeVisible();
    await expect(page.getByText("triage-reactor-0")).toBeVisible();

    await page.getByRole("button", { name: "Settings" }).click();
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
    await expect(page.getByText("Recent changes")).toBeVisible();
  });

  test("loads incidents and runbooks from backend API when credentials exist", async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem("paladin-auth-token", "test-token");
      window.localStorage.setItem("paladin-tenant-id", "tenant-live");
    });
    await page.route("http://127.0.0.1:9002/api/v1/incidents", async (route) => {
      await route.fulfill({
        status: 200,
        headers: {
          "access-control-allow-origin": "*",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          data: [
            {
              id: "live-1",
              status: "open",
              severity: "critical",
              title: "Live API outage",
              alert_count: 2,
              labels: { service: "edge-api", owner: "@live" },
              triage_result: {
                root_cause: "Edge proxy returned 502 from upstream.",
                confidence: 88,
                recommended_action: "Fail over edge-api to secondary region",
                evidence: ["502 spike", "upstream pool empty"],
              },
              created_at: new Date().toISOString(),
            },
          ],
        }),
      });
    });
    await page.route("http://127.0.0.1:9002/api/v1/runbooks", async (route) => {
      await route.fulfill({
        status: 200,
        headers: {
          "access-control-allow-origin": "*",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          data: [{ id: "rb-live", title: "Live failover", source: "github", embedded: true, updated_at: "2026-05-14T00:00:00Z" }],
        }),
      });
    });

    await page.goto(dashboardURL);

    await expect(page.getByText("Live backend")).toBeVisible();
    await expect(page.getByText("Live API outage")).toBeVisible();
    await page.getByRole("button", { name: "Runbooks" }).click();
    await expect(page.getByText("Live failover")).toBeVisible();
  });

  test("keeps mobile layout within viewport", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();

    await expect(page.getByRole("heading", { name: "Operations dashboard" })).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    expect(overflow).toBe(false);
  });
});

async function waitForDashboard() {
  const deadline = Date.now() + 20_000;
  while (Date.now() < deadline) {
    if (await canConnect()) return;
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error("dashboard dev server did not start");
}

function canConnect() {
  return new Promise<boolean>((resolve) => {
    const request = http.get(dashboardURL, (response) => {
      response.resume();
      resolve(response.statusCode === 200);
    });
    request.on("error", () => resolve(false));
    request.setTimeout(500, () => {
      request.destroy();
      resolve(false);
    });
  });
}
