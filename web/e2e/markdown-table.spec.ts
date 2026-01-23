import { expect, test } from '@playwright/test';
import { primeAuthSession } from './utils/auth';

const clearStorage = () => {
  window.localStorage.clear();
};

test.describe('Markdown tables', () => {
  test('renders tables from mkd artifacts', async ({ page }) => {
    await page.addInitScript(clearStorage);
    await primeAuthSession(page);
    await page.goto('/conversation?mockSSE=1');

    await page.waitForFunction(() =>
      Boolean((window as any).__ALEX_MOCK_STREAM__?.pushEvent)
    );

    await page.evaluate(() => {
      const controls = (window as any).__ALEX_MOCK_STREAM__;
      const markdown = [
        '# Table Preview',
        '',
        '| Name | Count |',
        '| --- | --- |',
        '| Alpha | 1 |',
        '| Beta | 2 |',
      ].join('\n');
      const uri = `data:text/markdown;base64,${btoa(unescape(encodeURIComponent(markdown)))}`;
      controls.pushEvent({
        event_type: 'workflow.result.final',
        final_answer: 'Added table preview.',
        total_iterations: 1,
        total_tokens: 80,
        stop_reason: 'final_answer',
        duration: 500,
        attachments: {
          'table.mkd': {
            name: 'table.mkd',
            description: 'Table Preview',
            media_type: 'text/plain',
            format: 'mkd',
            kind: 'artifact',
            uri,
          },
        },
      });
    });

    const previewButton = page.getByRole('button', { name: /preview table preview/i });
    await expect(previewButton).toBeVisible({ timeout: 60000 });
    await previewButton.click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const table = dialog.locator('table');
    await expect(table).toBeVisible();
    await expect(table.locator('td', { hasText: 'Alpha' })).toBeVisible();
  });
});
