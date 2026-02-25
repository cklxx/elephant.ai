import { expect, test } from '@playwright/test';
import path from 'path';
import { primeAuthSession } from './utils/auth';
import {
  capturePageScreenshot,
  shouldCaptureScreenshots,
} from './utils/screenshots';

const imageFixture = path.join(process.cwd(), 'e2e/fixtures/test-image.png');

test.describe('Task input image attachments', () => {
  test('allows attaching an image and renders it in the stream (mock mode)', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
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

    const fileInput = page.getByTestId('task-attachment-input');
    await fileInput.setInputFiles(imageFixture);

    const attachmentGallery = page.getByTestId('task-attachments');
    await expect(attachmentGallery).toBeVisible();
    await expect(
      attachmentGallery.getByRole('img', { name: 'test-image.png' })
    ).toBeVisible();
    await expect(textarea).toHaveValue(/\[test-image\.png\]/);

    await textarea.click();
    await textarea.press('End');
    await textarea.type(' Please review this image.');
    await textarea.press('Enter');

    const submittedInputEvent = page
      .getByTestId('event-workflow.input.received')
      .filter({ hasText: 'Please review this image.' })
      .first();
    await expect(submittedInputEvent).toBeVisible({ timeout: 15000 });
    await expect(submittedInputEvent.getByTestId('event-input-media')).toBeVisible();
    await expect(
      submittedInputEvent.getByRole('button', { name: /User upload|test-image/i }).first()
    ).toBeVisible();

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'image-attachment');
    }
  });
});
