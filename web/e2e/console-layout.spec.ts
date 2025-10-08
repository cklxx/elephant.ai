import { test, expect } from '@playwright/test';

const STORAGE_KEY = 'alex-session-storage';

test.describe('ALEX console layout', () => {
  test('renders console shell with empty state', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByText('ALEX 控制台')).toBeVisible();
    await expect(page.getByText('Operator Dashboard')).toBeVisible();
    await expect(page.getByText('历史会话', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: '新建对话' })).toBeVisible();

    await expect(page.getByText('目前还没有历史会话。')).toBeVisible();
    await expect(page.getByText('准备好接管你的任务')).toBeVisible();
    await expect(page.getByText('等待新的任务指令')).toBeVisible();
    await expect(page.getByText('Disconnected')).toBeVisible();

    const input = page.getByTestId('task-input');
    await expect(input).toHaveAttribute('placeholder', '请输入你想完成的任务或问题…');
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

    await expect(page.getByText(/会话 session/i)).toBeVisible();
    await expect(page.getByTestId('session-history-session-123456')).toBeVisible();

    await page.getByTestId('session-history-session-abcdef').click();

    await expect(page.getByTestId('session-history-session-abcdef')).toHaveAttribute('aria-current', 'true');
  });

  test('shows mock indicator when enabled', async ({ page }) => {
    await page.goto('/?mockSSE=1');

    await expect(page.getByTestId('mock-stream-indicator')).toContainText('Mock Stream Enabled');
  });
});
