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
  test('renders html, markdown, pptx, pdf, and image attachments with filters', async ({ page }) => {
    await bootstrapMockConversation(page);

    const documentTab = page.getByRole('tab', { name: /Document/i });
    await documentTab.click();

    await expect(page.getByText('Attachment filters')).toBeVisible({ timeout: 45000 });

    const artifactNames = [
      'Executive Review Slides',
      'Console Architecture Prototype',
      'Q3 Research Memo',
      'Latency Report',
    ];
    for (const name of artifactNames) {
      await expect(page.getByText(name).first()).toBeVisible();
    }

    await expect(page.getByAltText('Status Heatmap')).toBeVisible();
    await expect(
      page.getByTitle('Console Architecture Prototype preview')
    ).toBeVisible();

    const artifactsButton = page.getByRole('button', { name: 'Artifacts' }).first();
    await artifactsButton.click();
    await expect(page.getByText('Status Heatmap')).not.toBeVisible();

    const mediaButton = page.getByRole('button', { name: 'Media' }).first();
    await mediaButton.click();
    await expect(page.getByText('Status Heatmap')).toBeVisible();

    const formatSelect = page.getByLabel('Format');
    await formatSelect.selectOption('markdown');
    await expect(page.getByText('Q3 Research Memo')).toBeVisible();
    await expect(page.getByText('Executive Review Slides')).not.toBeVisible();
    await formatSelect.selectOption('all');

    const searchInput = page.getByLabel('Search');
    await searchInput.fill('latency');
    await expect(page.getByText('Latency Report')).toBeVisible();
    await expect(page.getByText('Executive Review Slides')).not.toBeVisible();
    await searchInput.fill('');

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'artifact-gallery');
    }
  });
});
