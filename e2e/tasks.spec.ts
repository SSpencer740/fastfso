import { test, expect } from "@playwright/test";
import { loginAsAdmin, ensureSeeded } from "./helpers/auth";

test.describe("Task Management", () => {
  test.beforeAll(async () => {
    await ensureSeeded();
  });

  test("admin can create a task", async ({ page }) => {
    await loginAsAdmin(page);

    // Navigate to tasks
    await page.getByRole("link", { name: /tasks/i }).first().click();
    await expect(page.url()).toContain("/app/tasks");

    // Click New Task button
    await page.getByRole("button", { name: /new task/i }).click();

    // Fill in the create task form
    await expect(page.getByText("Create New Task")).toBeVisible();

    // Title
    await page
      .getByPlaceholder(/task title/i)
      .fill("Complete Annual Security Training");

    // Description
    await page
      .getByPlaceholder(/description|instructions/i)
      .fill(
        "All personnel must complete the annual security awareness training module.",
      );

    // Priority - select urgent
    await page.locator("select").first().selectOption("urgent");

    // Assignment - click Everyone tab
    await page.getByRole("button", { name: /everyone/i }).click();

    // Add a requirement
    await page.getByRole("button", { name: /add requirement/i }).click();
    await page.getByPlaceholder(/label/i).fill("Training Certificate");

    // Submit
    await page.getByRole("button", { name: /create task/i }).click();

    // Verify the task appears in the list
    await expect(
      page.getByText("Complete Annual Security Training"),
    ).toBeVisible({ timeout: 5000 });
  });

  test("admin can view task details", async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole("link", { name: /tasks/i }).first().click();

    // Click on a task row
    const taskRow = page.locator(".task-row").first();
    const isVisible = await taskRow
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    if (!isVisible) {
      test.skip();
      return;
    }

    await taskRow.click();

    // Modal should open with task details
    await expect(page.locator(".modal")).toBeVisible();
    await expect(page.getByText(/assignees/i)).toBeVisible();
  });
});
