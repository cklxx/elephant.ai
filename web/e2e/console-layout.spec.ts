import { test, expect } from '@playwright/test';

const STORAGE_KEY = 'alex-session-storage';

test.describe('ALEX console layout', () => {
  test('renders console shell with empty state', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByText('ALEX Console')).toBeVisible();
    await expect(page.getByText('Operator Dashboard')).toBeVisible();
    await expect(page.getByText('Session history', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Start new conversation' })).toBeVisible();

    await expect(page.getByText('No sessions yet.')).toBeVisible();
    await expect(page.getByText('Ready to take on your work')).toBeVisible();
    await expect(page.getByText('Waiting for your task')).toBeVisible();
    await expect(page.getByText('Disconnected')).toBeVisible();

    const input = page.getByTestId('task-input');
    await expect(input).toHaveAttribute(
      'placeholder',
      'Describe the task or question you want to work on…'
    );
  });

  test('supports persisted sessions and selection', async ({ page }) => {
    await page.addInitScript(({ key }) => {
      const payload = {
        state: {
          currentSessionId: 'session-123456',
          sessionHistory: ['session-123456', 'session-abcdef'],
        },
        version: 0,
      };
      window.localStorage.setItem(key, JSON.stringify(payload));
    }, { key: STORAGE_KEY });

    await page.goto('/');

    await expect(page.getByText('Session sess…3456')).toBeVisible();
    await expect(page.getByTestId('session-history-session-123456')).toBeVisible();

    await page.getByTestId('session-history-session-abcdef').click();

    await expect(page.getByTestId('session-history-session-abcdef')).toHaveAttribute('aria-current', 'true');
  });

  test('shows mock indicator when enabled', async ({ page }) => {
    await page.goto('/?mockSSE=1');

    await expect(page.getByTestId('mock-stream-indicator')).toContainText('Mock stream enabled');
  });
});
