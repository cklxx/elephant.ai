import { test, expect } from '@playwright/test';
import { primeAuthSession } from './utils/auth';
import {
  capturePageScreenshot,
  shouldCaptureScreenshots,
} from './utils/screenshots';

const STORAGE_KEY = 'alex-session-storage';

test.describe('ALEX console layout', () => {
  test('renders console shell with empty state', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    await primeAuthSession(page);
    await page.goto('/conversation');

    const header = page.getByTestId('console-header-title');
    await expect(header).toBeVisible({ timeout: 60000 });
    const openSidebar = page.getByTestId('session-list-toggle');
    await expect(openSidebar).toBeVisible({ timeout: 60000 });
    await openSidebar.click();

    await expect(page.getByTestId('session-list-new')).toBeVisible();

    await expect(page.getByTestId('session-list-empty')).toBeVisible();
    await expect(page.getByTestId('conversation-empty-state')).toBeVisible();

    const input = page.getByTestId('task-input');
    await expect(input).toBeVisible();

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'console-empty-state');
    }
  });

  test('supports persisted sessions and selection', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    const storagePayload = JSON.stringify({
      state: {
        currentSessionId: 'session-123456',
        sessionHistory: ['session-123456', 'session-abcdef'],
        sessionLabels: {
          'session-123456': 'Primary workflow',
        },
      },
      version: 0,
    });
    await page.addInitScript(
      ({ key, payload }) => {
        window.localStorage.setItem(key, payload);
      },
      { key: STORAGE_KEY, payload: storagePayload }
    );
    await primeAuthSession(page);

    await page.goto('/conversation');
    const header = page.getByTestId('console-header-title');
    await expect(header).toBeVisible({ timeout: 60000 });
    const openSidebar = page.getByTestId('session-list-toggle');
    await expect(openSidebar).toBeVisible({ timeout: 60000 });
    await openSidebar.click();

    await expect(page.getByTestId('session-list')).toBeVisible();

    const primarySessionButton = page.locator(
      '[data-testid="session-list-item"][data-session-id="session-123456"]'
    );
    await expect(primarySessionButton).toBeVisible();

    const recentSessionButton = page.locator(
      '[data-testid="session-list-item"][data-session-id="session-abcdef"]'
    );
    await expect(recentSessionButton).toBeVisible();

    await recentSessionButton.click({ force: true });

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'console-persisted-session');
    }
  });

  test('supports mock stream mode', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    await primeAuthSession(page);
    await page.goto('/conversation?mockSSE=1');

    const header = page.getByTestId('console-header-title');
    await expect(header).toBeVisible({ timeout: 60000 });
    const openSidebar = page.getByTestId('session-list-toggle');
    await expect(openSidebar).toBeVisible({ timeout: 60000 });
    await openSidebar.click();

    await expect(page.getByTestId('session-list-empty')).toBeVisible();

    const input = page.getByTestId('task-input');
    await input.click();
    await page.keyboard.type('Mock stream task');
    await page.keyboard.press('Enter');

    await expect(page.locator('[data-testid="session-list-item"]')).toBeVisible({
      timeout: 15000,
    });
    await expect(page.getByTestId('event-workflow.input.received')).toBeVisible({ timeout: 15000 });
    await expect(page.getByTestId('console-header-title')).toHaveText(/.+/);

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'console-mock-stream');
    }
  });

  test('home route redirects to conversation view', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    await primeAuthSession(page);
    await page.goto('/');

    await expect(page).toHaveURL(/\/conversation$/);
    await expect(page.getByTestId('console-header-title')).toBeVisible({ timeout: 60000 });

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'console-redirect');
    }
  });
});
