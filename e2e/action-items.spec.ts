import { test, expect } from "@playwright/test";
import { loginAsAdmin, ensureSeeded } from "./helpers/auth";

test.describe("Action Items", () => {
  test.beforeAll(async () => {
    await ensureSeeded();
  });

  test("action items page loads", async ({ page }) => {
    await loginAsAdmin(page);

    // Navigate to action items
    await page.getByRole("link", { name: /action items/i }).click();
    await expect(page.url()).toContain("/app/action-items");

    // Should see the page header
    await expect(page.getByText("Action Items")).toBeVisible();
  });

  test("filters work on action items page", async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole("link", { name: /action items/i }).click();

    // The filter dropdowns should be visible
    await expect(page.locator(".filter-bar")).toBeVisible();

    // Search should work (type something and verify no crash)
    await page.getByPlaceholder(/search/i).fill("test search");
    // Should not crash - page should still be functional
    await expect(page.locator(".task-list")).toBeVisible();
  });
});
