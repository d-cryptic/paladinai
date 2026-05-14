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
