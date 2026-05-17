import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Comments", () => {
  let api: TestApiClient;
  let issue: Awaited<ReturnType<TestApiClient["createIssue"]>>;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    issue = await api.createIssue("E2E Comment Test " + Date.now());
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("can add a comment on an issue", async ({ page }) => {
    // Wait for issues to load and click first one. `*=` matches both legacy
    // `/issues/{id}` and URL-refactored `/{slug}/issues/{id}` hrefs.
    const issueLink = page.locator(`a[href$="/issues/${issue.id}"]`);
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();
    await page.waitForURL(/\/issues\/[\w-]+/);

    // Wait for issue detail to load
    await expect(page.getByText(issue.title).first()).toBeVisible();

    // Type a comment
    const commentText = "E2E comment " + Date.now();
    const commentInput = page.getByRole("textbox", {
      name: "Leave a comment...",
    });
    await commentInput.fill(commentText);

    // Submit the comment
    await page.getByRole("button", { name: "Send" }).first().click();

    // Comment should appear in the activity section
    await expect(page.locator(`text=${commentText}`)).toBeVisible({
      timeout: 5000,
    });
  });

  test("comment submit button is disabled when empty", async ({ page }) => {
    const issueLink = page.locator(`a[href$="/issues/${issue.id}"]`);
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();
    await page.waitForURL(/\/issues\/[\w-]+/);

    await expect(page.getByText(issue.title).first()).toBeVisible();

    // Submit button should be disabled when input is empty
    const submitBtn = page.getByRole("button", { name: "Send" }).first();
    await expect(submitBtn).toBeDisabled();
  });
});
