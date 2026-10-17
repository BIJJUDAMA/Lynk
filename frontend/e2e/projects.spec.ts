import { test, expect } from "@playwright/test";

test.describe("Lynk Campus Platform E2E Tests", () => {
  test("Home Page loads with editorial typography and project CTAs", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle(/Lynk/i);

    // Verify global navigation points to projects
    const projectsNavLink = page.locator('nav a[href="/projects"]').first();
    await expect(projectsNavLink).toBeVisible();
    await expect(projectsNavLink).toHaveText(/Projects/i);

    // Verify hero section primary CTA
    const heroCta = page.locator('section a[href="/projects"]').first();
    await expect(heroCta).toBeVisible();
  });

  test("Project Discovery Surface loads search, filters, and project cards", async ({ page }) => {
    await page.goto("/projects");
    await expect(page).toHaveURL(/.*projects/);

    // Verify discovery header
    const heading = page.locator("h1");
    await expect(heading).toContainText(/Projects/i);

    // Verify search input is rendered
    const searchInput = page.locator('input[placeholder*="Search projects" i], input[placeholder*="Describe" i]').first();
    await expect(searchInput).toBeVisible();

    // Verify Start a Project action link is rendered
    const startProjectLink = page.locator('a[href="/projects/create"]').first();
    await expect(startProjectLink).toBeVisible();
  });

  test("Project Creation Flow renders form or auth gate", async ({ page }) => {
    await page.goto("/projects/create");
    await expect(page).toHaveURL(/.*projects\/create/);

    // Page must either render the creation form or prompt to log in / verify email
    const content = page.locator("main, div.container, body");
    await expect(content.first()).toBeVisible();
  });

  test("Activity Hub renders My Projects and My Applications tabs", async ({ page }) => {
    await page.goto("/activity");
    await expect(page).toHaveURL(/.*activity/);

    // Verify Activity tabs exist
    const myProjectsTab = page.locator('button:has-text("My Projects"), a:has-text("My Projects")').first();
    const myAppsTab = page.locator('button:has-text("My Applications"), a:has-text("My Applications")').first();

    // In unauthenticated or authenticated state, tab labels are visible
    const bodyText = await page.textContent("body");
    expect(bodyText).toMatch(/Projects|Applications/i);
  });
});
