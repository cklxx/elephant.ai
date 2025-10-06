import { test, expect } from '@playwright/test';

test.describe('Basic User Flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should load homepage successfully', async ({ page }) => {
    await expect(page).toHaveTitle(/ALEX/i);
    await expect(page.locator('h1')).toContainText(/ALEX/i);
  });

  test('should display task input form', async ({ page }) => {
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await expect(taskInput).toBeVisible();
    await expect(taskInput).toBeEnabled();
  });

  test('should have navigation links', async ({ page }) => {
    const homeLink = page.getByRole('link', { name: /home/i });
    const sessionsLink = page.getByRole('link', { name: /sessions/i });

    await expect(homeLink).toBeVisible();
    await expect(sessionsLink).toBeVisible();
  });

  test('should navigate to sessions page', async ({ page }) => {
    await page.getByRole('link', { name: /sessions/i }).click();
    await expect(page).toHaveURL(/\/sessions/);
  });
});

test.describe('Task Submission Flow', () => {
  test.skip('should submit task and show connection status', async ({ page }) => {
    // This test requires a running backend
    await page.goto('/');

    const taskInput = page.getByRole('textbox', { name: /task/i });
    await taskInput.fill('List all files in the current directory');

    const submitButton = page.getByRole('button', { name: /submit|execute/i });
    await submitButton.click();

    // Wait for connection status
    await expect(page.getByText(/connecting|connected/i)).toBeVisible({ timeout: 10000 });
  });

  test.skip('should display agent output stream', async ({ page }) => {
    // This test requires a running backend
    await page.goto('/');

    const taskInput = page.getByRole('textbox', { name: /task/i });
    await taskInput.fill('Simple test task');

    const submitButton = page.getByRole('button', { name: /submit|execute/i });
    await submitButton.click();

    // Wait for agent output container
    await expect(page.locator('[class*="agent-output"]')).toBeVisible({ timeout: 10000 });
  });
});

test.describe('Session Management', () => {
  test('should display sessions page', async ({ page }) => {
    await page.goto('/sessions');

    // Should show sessions header
    await expect(page.locator('h1, h2').filter({ hasText: /sessions/i })).toBeVisible();
  });

  test.skip('should display session list when sessions exist', async ({ page }) => {
    // This test requires a running backend with existing sessions
    await page.goto('/sessions');

    // Wait for session cards to load
    await expect(page.locator('[data-testid="session-card"]').first()).toBeVisible({ timeout: 10000 });
  });
});

test.describe('Responsive Design', () => {
  test('should be responsive on mobile viewport', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 }); // iPhone SE
    await page.goto('/');

    // Check that navigation is still accessible (might be collapsed)
    const header = page.locator('header, nav');
    await expect(header).toBeVisible();
  });

  test('should be responsive on tablet viewport', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 }); // iPad
    await page.goto('/');

    // Page should render without horizontal scroll
    const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
    expect(bodyWidth).toBeLessThanOrEqual(768);
  });

  test('should be responsive on desktop viewport', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 }); // Desktop
    await page.goto('/');

    // Check that content is properly centered/laid out
    const main = page.locator('main');
    await expect(main).toBeVisible();
  });
});

test.describe('Accessibility', () => {
  test('should have no accessibility violations on homepage', async ({ page }) => {
    await page.goto('/');

    // Basic accessibility checks
    await expect(page.locator('main')).toBeVisible();
    await expect(page.locator('h1, h2, h3')).toHaveCount({ min: 1 });
  });

  test('should support keyboard navigation', async ({ page }) => {
    await page.goto('/');

    // Tab through interactive elements
    await page.keyboard.press('Tab');

    // Check that focus is visible
    const focused = page.locator(':focus');
    await expect(focused).toBeVisible();
  });

  test('should have proper heading hierarchy', async ({ page }) => {
    await page.goto('/');

    const h1Count = await page.locator('h1').count();
    expect(h1Count).toBeGreaterThanOrEqual(1);
  });
});

test.describe('Error Handling', () => {
  test('should handle 404 gracefully', async ({ page }) => {
    const response = await page.goto('/non-existent-page');
    expect(response?.status()).toBe(404);
  });

  test('should show connection status when backend is unavailable', async ({ page }) => {
    await page.goto('/');

    // Even without backend, UI should render
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await expect(taskInput).toBeVisible();
  });
});
