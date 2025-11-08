import { test, expect } from '@playwright/test';

const STORAGE_KEY = 'alex-session-storage';

test.describe('ALEX console layout', () => {
  test('renders console shell with empty state', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    await page.goto('/conversation');

    await expect(
      page.getByRole('heading', { name: 'Start new run', level: 1 })
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: 'New Session', exact: true })
    ).toBeVisible();

    await expect(page.getByText('No sessions yet')).toBeVisible();
    await expect(page.getByText('Ready to start')).toBeVisible();
    await expect(page.getByText('Send a task to begin.')).toBeVisible();
    await expect(page.getByText('Waiting', { exact: true })).toBeVisible();

    const input = page.locator('textarea[placeholder="Describe your task…"]');
    await expect(input).toBeVisible();
  });

  test('supports persisted sessions and selection', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    const storagePayload = JSON.stringify({
      state: {
        currentSessionId: 'session-123456',
        sessionHistory: ['session-abcdef'],
        pinnedSessions: ['session-123456'],
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

    await page.goto('/conversation');

    await expect(page.getByText('Pinned')).toBeVisible();
    await expect(page.getByText('Recent')).toBeVisible();

    const pinnedSession = page.getByRole('button', { name: /Primary workflow/ });
    await expect(pinnedSession).toBeVisible();

    const recentSession = page.getByRole('button', { name: 'sess…cdef' });
    await expect(recentSession).toBeVisible();

    await recentSession.click();

    await expect(
      page.getByRole('heading', { level: 1 })
    ).toHaveText(/sess…cdef/i);
  });

  test('supports mock stream mode', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    await page.goto('/conversation?mockSSE=1');

    await expect(
      page.getByRole('heading', { name: 'Start new run', level: 1 })
    ).toBeVisible();
    await expect(page.getByText('No sessions yet')).toBeVisible();

    const input = page.locator('textarea[placeholder="Describe your task…"]');
    await input.click();
    await page.keyboard.type('Mock stream task');
    await page.keyboard.press('Enter');

    await expect(page.getByText('Recent')).toBeVisible({ timeout: 15000 });
    await expect(
      page.getByRole('button', { name: /Understanding task requirements/i })
    ).toBeVisible({ timeout: 15000 });
    await expect(
      page.getByRole('heading', { level: 1, name: /Understanding task requirements/i })
    ).toBeVisible();
    await expect(
      page.locator('textarea[placeholder="Add detail…"]')
    ).toBeVisible();
  });

  test('home route redirects to conversation view', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    await page.goto('/');

    await expect(page).toHaveURL(/\/conversation$/);
    await expect(
      page.getByRole('heading', { name: 'Start new run', level: 1 })
    ).toBeVisible();
  });
});
