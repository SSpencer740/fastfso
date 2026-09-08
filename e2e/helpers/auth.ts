import { type Page, expect } from "@playwright/test";

/**
 * Log in as the seeded admin user (admin@example.com / admin).
 * This navigates through the identifier-first login flow:
 * 1. Enter email on login page
 * 2. Enter password
 * 3. Select tenant (or admin panel)
 * 4. Wait for dashboard
 */
export async function loginAsAdmin(page: Page) {
  await page.goto("/login");

  // Step 1: Enter email
  await page.getByPlaceholder(/email/i).fill("admin@example.com");
  await page.getByRole("button", { name: /continue|next|sign in/i }).click();

  // Step 2: Enter password
  await page.getByPlaceholder(/password/i).fill("admin");
  await page.getByRole("button", { name: /sign in|log in|continue/i }).click();

  // Step 3: Select tenant (click the first tenant card if shown)
  // The admin may see a tenant selection page or go straight to dashboard
  const tenantCard = page.locator(".tenant-card").first();
  const hasTenants = await tenantCard
    .isVisible({ timeout: 3000 })
    .catch(() => false);
  if (hasTenants) {
    await tenantCard.click();
  }

  // Wait for app shell to load
  await expect(page.locator(".app-shell")).toBeVisible({ timeout: 10000 });
}

/**
 * Seed the database. Playwright runs with cwd=e2e/, so repo root is "..".
 */
export async function ensureSeeded() {
  const { execSync } = await import("child_process");
  execSync("./dev.sh seed", {
    cwd: "..",
    stdio: "pipe",
    timeout: 15000,
  });
}
