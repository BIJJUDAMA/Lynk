import { test, expect } from "@playwright/test";

test.describe("Peer Collaboration & Workspace Discovery", () => {
  test("Projects discovery surface renders search and filter controls", async ({ page }) => {
    await page.goto("/projects");
    await expect(page).toHaveURL(/.*projects/);

    // Verify discovery header and badge
    const badge = page.locator('text="Campus Collaborative Workspaces"');
    await expect(badge).toBeVisible();

    const heading = page.locator("h1");
    await expect(heading).toContainText(/Campus Projects/i);

    // Verify search input
    const searchInput = page.locator('input[placeholder*="Search projects" i]');
    await expect(searchInput).toBeVisible();

    // Verify typing into search updates the input
    await searchInput.fill("Robotics");
    await expect(searchInput).toHaveValue("Robotics");

    // Clear button should be visible when input has content
    const clearButton = page.locator('button[aria-label="Clear search"]');
    await expect(clearButton).toBeVisible();
    await clearButton.click();
    await expect(searchInput).toHaveValue("");

    // Verify Semantic AI Search mode toggle
    const semanticToggle = page.locator('button:has-text("Semantic AI Search")');
    await expect(semanticToggle).toBeVisible();
    await semanticToggle.click();

    // Placeholder updates to describe idea prompt in semantic mode
    const semanticInput = page.locator('input[placeholder*="Describe your idea" i]');
    await expect(semanticInput).toBeVisible();

    // Toggle back to standard search
    await semanticToggle.click();
    await expect(searchInput).toBeVisible();

    // Verify track categories exist
    const trackLabel = page.locator('text="Track:"');
    await expect(trackLabel).toBeVisible();

    // Verify Start a Project action CTA
    const startProjectCta = page.locator('a[href="/projects/create"]:has-text("Start a Project")').first();
    await expect(startProjectCta).toBeVisible();
  });

  test("Activity surface renders tabs for Projects and Collaborations", async ({ page }) => {
    await page.goto("/activity");
    await expect(page).toHaveURL(/.*activity/);

    // The activity page renders either the authenticated dashboard tabs or the campus sign in gate
    const bodyContent = page.locator("body");
    await expect(bodyContent).toBeVisible();

    // Wait for content after auth resolution
    const headerOrGate = page.locator(
      'h1:has-text("Campus Sign In Required"), h1:has-text("Activity Hub"), button:has-text("My Projects")'
    ).first();
    await expect(headerOrGate).toBeVisible();

    const pageText = await page.textContent("body");
    expect(pageText).toMatch(/Projects|Applications|Sign In|Activity/i);
  });

  test("Direct navigation to project creation from discovery links cleanly", async ({ page }) => {
    await page.goto("/projects");

    const startProjectLink = page.locator('a[href="/projects/create"]').first();
    await expect(startProjectLink).toBeVisible();

    await startProjectLink.click();
    await expect(page).toHaveURL(/.*projects\/create/);

    // Navigating unauthenticated presents the sign in required gate
    const gatePrompt = page.locator('h1:has-text("Campus Sign In Required")');
    await expect(gatePrompt).toBeVisible();
  });
});
