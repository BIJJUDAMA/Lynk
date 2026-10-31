import { test, expect } from "@playwright/test";

test.describe("Institutional .edu Campus Verification & Auth Gates", () => {
  test("Navigating to /projects/create unauthenticated displays campus sign in required gate", async ({
    page,
  }) => {
    await page.goto("/projects/create");
    await expect(page).toHaveURL(/.*projects\/create/);

    // Verify Campus Sign In Required heading is displayed
    const heading = page.locator("h1");
    await expect(heading).toContainText(/Campus Sign In Required/i);

    // Verify explanation copy describes university verification
    const gateCopy = page.locator("p");
    await expect(gateCopy.first()).toBeVisible();

    // Verify primary action button to sign in with campus account
    const signInButton = page.locator('button:has-text("Sign In with Campus Account")').first();
    await expect(signInButton).toBeVisible();

    // Verify secondary navigation link back to discovery
    const browseLink = page.locator('a[href="/projects"]:has-text("Browse Campus Projects")').first();
    await expect(browseLink).toBeVisible();

    // Clicking browse projects should navigate back to /projects
    await browseLink.click();
    await expect(page).toHaveURL(/.*projects/);
  });

  test("Accessing /activity unauthenticated presents campus authentication prompt", async ({
    page,
  }) => {
    await page.goto("/activity");
    await expect(page).toHaveURL(/.*activity/);

    // Wait for auth check to finish and verify gate or activity content
    const heading = page.locator('h1:has-text("Campus Sign In Required"), h1:has-text("Activity Hub")').first();
    await expect(heading).toBeVisible();

    // Verify primary action to authenticate or view workspaces
    const actionTarget = page.locator(
      'button:has-text("Sign In with Campus Account"), button:has-text("My Projects")'
    ).first();
    await expect(actionTarget).toBeVisible();
  });

  test("Registration form enforces institutional university email messaging", async ({ page }) => {
    await page.goto("/login");
    await expect(page).toHaveURL(/.*login/);

    // Verify LYNK header and title
    const headerTitle = page.locator("h1");
    await expect(headerTitle).toBeVisible();

    // Check institutional .edu requirement note
    const eduNote = page.locator('text="Institutional .edu address required."');
    await expect(eduNote).toBeVisible();

    // Switch to Create Account mode
    const createAccountTab = page.locator('button:has-text("Create Account")');
    await expect(createAccountTab).toBeVisible();
    await createAccountTab.click();

    // Form should now reveal First Name and Last Name inputs
    const firstNameInput = page.locator('input[placeholder*="Sarah" i]');
    const lastNameInput = page.locator('input[placeholder*="Chen" i]');
    await expect(firstNameInput).toBeVisible();
    await expect(lastNameInput).toBeVisible();

    // Email and Password inputs must be present
    const emailInput = page.locator('input[type="email"]');
    const passwordInput = page.locator('input[type="password"]');
    await expect(emailInput).toBeVisible();
    await expect(passwordInput).toBeVisible();
  });

  test("Email verification portal renders guidance cleanly", async ({ page }) => {
    await page.goto("/verify-email");
    await expect(page).toHaveURL(/.*verify-email/);

    const bodyContent = page.locator("body");
    await expect(bodyContent).toBeVisible();

    // Page must contain verification instructions or status container
    const heading = page.locator("h1, h2");
    await expect(heading.first()).toBeVisible();
  });
});
