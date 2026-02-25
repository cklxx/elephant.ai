import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';
import { primeAuthSession } from './utils/auth';
import {
  capturePageScreenshot,
  shouldCaptureScreenshots,
} from './utils/screenshots';

const clearStorage = () => {
  window.localStorage.clear();
};

async function bootstrapMockConversation(page: Page) {
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
  await textarea.fill('Show me every artifact preview type.');
  await textarea.press('Enter');
}

test.describe('Artifact previews in mock mode', () => {
  test('renders right-panel attachments for mock tool outputs', async ({ page }) => {
    await bootstrapMockConversation(page);

    const rightPanelToggle = page.getByTestId('right-panel-toggle');
    await expect(rightPanelToggle).toBeVisible({ timeout: 60000 });
    await rightPanelToggle.click();

    const attachmentPanel = page.locator('[data-testid="attachment-panel"]:visible').first();
    await expect(attachmentPanel).toBeVisible({ timeout: 60000 });
    await expect(attachmentPanel.getByText('Console Architecture Prototype').first()).toBeVisible();
    await expect(attachmentPanel.getByText('UX Snapshot mock').first()).toBeVisible();
    await expect(
      attachmentPanel.getByRole('button', { name: /Preview Console Architecture Prototype/i }).first()
    ).toBeVisible();
    await expect(
      attachmentPanel.getByRole('button', { name: /UX Snapshot mock/i }).first()
    ).toBeVisible();

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'artifact-gallery');
    }
  });
});
