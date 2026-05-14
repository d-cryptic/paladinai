import { expect, test } from "@playwright/test";
import path from "node:path";
import { pathToFileURL } from "node:url";

const dashboardURL = pathToFileURL(
  path.resolve(__dirname, "../../../ui/dashboard/index.html"),
).toString();

test.describe("local dashboard", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(dashboardURL);
  });

  test("renders incident command overview", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Operations dashboard" })).toBeVisible();
    await expect(page.getByText("Active incidents")).toBeVisible();
    await expect(page.getByText("Payment DB connection exhaustion")).toBeVisible();
    await expect(page.getByText("Connection pool saturation")).toBeVisible();
  });

  test("filters incidents and updates selected detail", async ({ page }) => {
    await page.getByLabel("Severity filter").selectOption("P2");

    await expect(page.getByText("Auth latency spike")).toBeVisible();
    await expect(page.getByText("Payment DB connection exhaustion")).toHaveCount(0);
    await expect(page.getByRole("heading", { name: /INC-2039/ })).toBeVisible();
    await expect(page.locator("#detail-action")).toHaveText("Scale auth-cache read replicas");
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

  test("keeps mobile layout within viewport", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();

    await expect(page.getByRole("heading", { name: "Operations dashboard" })).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    expect(overflow).toBe(false);
  });
});
