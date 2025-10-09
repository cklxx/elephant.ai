import { test, expect } from '@playwright/test';

const STORAGE_KEY = 'alex-session-storage';

test.describe('ALEX console layout', () => {
  test('renders console shell with empty state', async ({ page }) => {
    await page.goto('/conversation');

    await expect(page.getByText('Conversation')).toBeVisible();
    await expect(page.getByText('Start new run')).toBeVisible();
    await expect(page.getByText('Session history', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Reset' })).toBeVisible();

    await expect(page.getByText('No sessions')).toBeVisible();
    await expect(page.getByText('Ready to start')).toBeVisible();
    await expect(page.getByText('Waiting', { exact: true })).toBeVisible();
    await expect(page.getByText('Offline')).toBeVisible();

    const input = page.getByTestId('task-input');
    await expect(input).toHaveAttribute(
      'placeholder',
      'Describe your task…'
    );
  });

  test('supports persisted sessions and selection', async ({ page }) => {
    await page.addInitScript(({ key }) => {
      const payload = {
        state: {
          currentSessionId: 'session-123456',
          sessionHistory: ['session-abcdef'],
          pinnedSessions: ['session-123456'],
          sessionLabels: {
            'session-123456': 'Primary workflow',
          },
        },
        version: 0,
      };
      window.localStorage.setItem(key, JSON.stringify(payload));
    }, { key: STORAGE_KEY });

    await page.goto('/conversation');

    await expect(page.getByText('Pinned')).toBeVisible();
    await expect(page.getByText('Recent')).toBeVisible();
    await expect(page.getByTestId('session-history-session-123456')).toContainText(
      'Primary workflow'
    );
    await expect(page.getByTestId('session-history-session-123456')).toBeVisible();

    await page.getByTestId('session-history-session-abcdef').click();

    await expect(page.getByTestId('session-history-session-abcdef')).toHaveAttribute('aria-current', 'true');
  });

  test('shows mock indicator when enabled', async ({ page }) => {
    await page.goto('/conversation?mockSSE=1');

    await expect(page.getByTestId('mock-stream-indicator')).toContainText('Mock stream enabled');
  });

  test('home hero routes to conversation view', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByText('Research workspace')).toBeVisible();

    await page.getByRole('link', { name: 'Open conversation view' }).click();
    await expect(page).toHaveURL(/\/conversation/);
  });
});
