import { expect, test } from '@playwright/test';
import path from 'path';

const imageFixture = path.join(process.cwd(), 'e2e/fixtures/test-image.png');

test.describe('Task input image attachments', () => {
  test('allows attaching an image and renders it in the stream (mock mode)', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    await page.goto('/conversation?mockSSE=1');

    const newConversationButton = page.getByRole('button', { name: 'Start new conversation' });
    await expect(newConversationButton).toBeVisible();
    await newConversationButton.click();

    const textarea = page.getByTestId('task-input');
    await expect(textarea).toBeVisible();

    const fileInput = page.locator('input[type="file"][accept="image/*"]');
    await fileInput.setInputFiles(imageFixture);

    const attachmentGallery = page.getByTestId('task-attachments');
    await expect(attachmentGallery).toBeVisible();
    await expect(attachmentGallery.getByRole('img', { name: 'test-image.png' })).toBeVisible();
    await expect(textarea).toHaveValue(/\[test-image\.png\]/);

    await textarea.click();
    await textarea.press('End');
    await textarea.type(' Please review this image.');
    await textarea.press('Enter');

    await expect(page.getByRole('img', { name: 'test-image.png' })).toBeVisible();
  });
});
