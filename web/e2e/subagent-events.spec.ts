import { expect, test } from '@playwright/test';
import { primeAuthSession } from './utils/auth';

const STORAGE_KEY = 'alex-session-storage';

const clearStorage = () => {
  window.localStorage.clear();
};

test.describe('Subagent event rendering', () => {
  test('renders mock subagent tool output and headers', async ({ page }) => {
    await page.addInitScript(clearStorage);
    await primeAuthSession(page);
    await page.goto('/conversation?mockSSE=1');

    const openSidebar = page.getByTestId('session-list-toggle');
    await expect(openSidebar).toBeVisible({ timeout: 60000 });
    await openSidebar.click();

    const newConversationButton = page.getByTestId('session-list-new');
    await expect(newConversationButton).toBeVisible();
    await newConversationButton.click();

    const textarea = page.getByTestId('task-input');
    await expect(textarea).toBeVisible();
    await textarea.fill('Review nested tool call output for subagents.');
    await textarea.press('Enter');

    await expect(page.getByText('Subagent Task 1/2')).toBeVisible({ timeout: 30000 });
    await expect(
      page.getByText('Research comparable console UX patterns')
    ).toBeVisible();
    await expect(page.getByText('Parallel ×2').first()).toBeVisible();

    await expect(page.getByText('Subagent Task 2/2')).toBeVisible();
    await expect(
      page.getByText('Inspect tool output rendering implementation')
    ).toBeVisible();

    const subagentCompletions = page.getByTestId('event-subagent-task_complete');
    await expect(subagentCompletions.first()).toContainText('Validated layout guidance');
    await expect(subagentCompletions.nth(1)).toContainText('Confirmed ToolOutputCard handles metadata');
  });
});
