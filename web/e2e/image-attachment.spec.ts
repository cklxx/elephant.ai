import { expect, test } from '@playwright/test';
import fs from 'fs';
import os from 'os';
import path from 'path';

const imageFileName = 'mock-image.png';
const pngBase64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAGgwJ/lzE6SAAAAABJRU5ErkJggg==';
let imageFixturePath: string | undefined;

function ensureImageFixture(): string {
  if (!imageFixturePath) {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'alex-e2e-'));
    imageFixturePath = path.join(tempDir, imageFileName);
    fs.writeFileSync(imageFixturePath, Buffer.from(pngBase64, 'base64'));
  }
  return imageFixturePath;
}

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
    const imageFixture = ensureImageFixture();
    await fileInput.setInputFiles(imageFixture);

    const attachmentGallery = page.getByTestId('task-attachments');
    await expect(attachmentGallery).toBeVisible();
    await expect(attachmentGallery.getByRole('img', { name: imageFileName })).toBeVisible();

    const escapedImageName = imageFileName.replace(/\./g, '\\.');
    await expect(textarea).toHaveValue(new RegExp(`\\[${escapedImageName}\\]`));

    await textarea.click();
    await textarea.press('End');
    await textarea.type(' Please review this image.');
    await textarea.press('Enter');

    await expect(page.getByText(`[${imageFileName}]`)).toBeVisible();
  });
});
