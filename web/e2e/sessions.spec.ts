import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    const session = {
      accessToken: 'seed-token',
      accessExpiry: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
      refreshExpiry: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
      user: {
        id: 'user-1',
        email: 'session@example.com',
        displayName: 'Session User',
        pointsBalance: 0,
        subscription: {
          tier: 'free',
          monthlyPriceCents: 0,
          expiresAt: null,
          isPaid: false,
        },
        photoURL: null,
      },
    };
    window.localStorage.setItem('alex.console.auth', JSON.stringify(session));
  });

  await page.route('**/api/sessions', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        sessions: [
          {
            id: 'session-123',
            created_at: '2024-10-01T12:00:00.000Z',
            updated_at: new Date().toISOString(),
            task_count: 2,
            last_task: 'task-99',
          },
        ],
      }),
    });
  });
});

test('shows sessions returned from the API', async ({ page }) => {
  await page.goto('/sessions');

  await expect(page.getByText('session-123')).toBeVisible();
  await expect(page.getByText('task-99')).toBeVisible();
  await expect(page.getByText(/2 tasks/i)).toBeVisible();
});
