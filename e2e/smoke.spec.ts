import { test, expect } from "@playwright/test";
import { loginAsAdmin, ensureSeeded } from "./helpers/auth";

test.describe("Smoke Tests", () => {
  test.beforeAll(async () => {
    await ensureSeeded();
  });

  test("login page loads", async ({ page }) => {
    await page.goto("/login");
    await expect(page).toHaveTitle(/fastFSO/);
    // Should see the login form
    await expect(page.getByPlaceholder(/email/i)).toBeVisible();
  });

  test("login flow works", async ({ page }) => {
    await loginAsAdmin(page);
    // Should be on a page within the app
    await expect(page.url()).toContain("/app/");
  });

  test("sidebar navigation works", async ({ page }) => {
    await loginAsAdmin(page);

    // Click Dashboard
    await page.getByRole("link", { name: /dashboard/i }).click();
    await expect(page.url()).toContain("/app/dashboard");

    // Click Tasks (admin sees "Manage Tasks")
    const tasksLink = page.getByRole("link", { name: /tasks/i }).first();
    await tasksLink.click();
    await expect(page.url()).toContain("/app/tasks");

    // Click Reports
    await page.getByRole("link", { name: /reports/i }).click();
    await expect(page.url()).toContain("/app/reports");

    // Click Travel
    await page.getByRole("link", { name: /travel/i }).click();
    await expect(page.url()).toContain("/app/travel");

    // Click Visit Requests
    await page.getByRole("link", { name: /visit/i }).click();
    await expect(page.url()).toContain("/app/visits");

    // Click Wiki
    await page.getByRole("link", { name: /wiki/i }).click();
    await expect(page.url()).toContain("/app/wiki");

    // Click Settings (in footer)
    await page.getByRole("link", { name: /settings/i }).click();
    await expect(page.url()).toContain("/app/settings");
  });

  test("logout works", async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole("button", { name: /sign out/i }).click();
    await expect(page.url()).toContain("/login");
  });
});
