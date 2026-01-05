import { expect, test } from '@playwright/test';
import { primeAuthSession } from './utils/auth';

const clearStorage = () => {
  window.localStorage.clear();
};

test.describe('Attachment deduplication', () => {
  test('does not repeat tool attachments inside the final summary', async ({ page }) => {
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
    await textarea.fill('Validate attachment deduplication in final summaries.');
    await textarea.press('Enter');

    const attachmentToolEvent = page
      .getByTestId('event-workflow.tool.completed')
      .filter({ hasText: 'Planner/ReAct view layout and task/action mapping.' })
      .first();

    await expect(attachmentToolEvent).toBeVisible({ timeout: 60000 });
    await attachmentToolEvent.getByTestId('tool-output-header').click();
    await expect(attachmentToolEvent.getByText('UX Snapshot')).toBeVisible();
    await expect(attachmentToolEvent.getByText('Console Architecture Prototype')).toBeVisible();

    const finalSummary = page
      .getByTestId('event-workflow.result.final')
      .filter({ hasText: 'Planner/ReAct 架构展示与工具详情联动。' })
      .first();

    await expect(finalSummary).toBeVisible({ timeout: 60000 });
    await expect(finalSummary.getByText('UX Snapshot')).toHaveCount(0);
    await expect(finalSummary.getByText('Console Architecture Prototype')).toHaveCount(0);
    await expect(
      finalSummary.locator('img[alt*="UX Snapshot"], img[alt*="Console Architecture Prototype"]'),
    ).toHaveCount(0);
  });
});
