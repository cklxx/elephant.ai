import { expect, test } from '@playwright/test';
import { primeAuthSession } from './utils/auth';
import {
  capturePageScreenshot,
  shouldCaptureScreenshots,
} from './utils/screenshots';

const clearStorage = () => {
  window.localStorage.clear();
};

test.describe('Subagent event rendering', () => {
  test('renders mock subagent thread cards and nested outputs', async ({ page }) => {
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

    const threads = page.getByTestId('subagent-thread');
    await expect(threads).toHaveCount(2, { timeout: 60000 });

    const firstThread = threads.first();
    await expect(firstThread).toContainText('Subagent: gather docs');
    await expect(firstThread).toContainText('Parallel ×2');
    await firstThread.getByRole('button', { name: /Toggle .* details/i }).click();
    await expect(firstThread).toContainText('Found docs and examples.');
    await expect(firstThread).toContainText('Subagent summary: gathered protocol constraints and UI levels.');

    const secondThread = threads.nth(1);
    await expect(secondThread).toContainText('Subagent: verify UI');
    await expect(secondThread).toContainText('Parallel ×2');
    await secondThread.getByRole('button', { name: /Toggle .* details/i }).click();
    await expect(secondThread).toContainText('Reviewed EventLine implementation.');
    await expect(secondThread).toContainText('Subagent summary: UI renders Goal/Task/Log correctly; fonts consistent.');

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'subagent-events');
    }
  });
});
