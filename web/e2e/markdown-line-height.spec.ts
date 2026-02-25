import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';
import { primeAuthSession } from './utils/auth';

const clearStorage = () => {
  window.localStorage.clear();
};

async function bootstrapMockSession(page: Page) {
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
  await textarea.fill('Bootstrap markdown spacing checks.');
  await textarea.press('Enter');

  await expect(page.getByTestId('event-ui-plan')).toBeVisible({ timeout: 15000 });
}

test.describe('Markdown spacing', () => {
  test('uses compact line height in final summary markdown', async ({ page }) => {
    await bootstrapMockSession(page);

    await page.waitForFunction(() =>
      Boolean((window as any).__ALEX_MOCK_STREAM__?.pushEvent)
    );

    await page.evaluate(() => {
      const controls = (window as any).__ALEX_MOCK_STREAM__;
      controls.pushEvent({
        event_type: 'workflow.result.final',
        final_answer:
          'Spacing audit:\n- First line\n- Second line\n- Third line',
        total_iterations: 1,
        total_tokens: 80,
        stop_reason: 'final_answer',
        duration: 500,
      });
    });

    const summaryEvent = page
      .getByTestId('event-workflow.result.final')
      .filter({ hasText: 'Spacing audit' })
      .first();

    await expect(summaryEvent).toBeVisible({ timeout: 60000 });

    const metrics = await summaryEvent
      .locator('.markdown-body__content .whitespace-pre-wrap')
      .first()
      .evaluate((el: HTMLElement) => {
        const style = window.getComputedStyle(el);
        const fontSize = parseFloat(style.fontSize || '16');
        const lineHeight = parseFloat(style.lineHeight || '0');
        return {
          fontSize,
          lineHeight,
          ratio: lineHeight / fontSize,
        };
      });

    expect(metrics.ratio).toBeLessThanOrEqual(1.4);
  });

  test('keeps markdown preview headings compact', async ({ page }) => {
    await bootstrapMockSession(page);

    await page.waitForFunction(() =>
      Boolean((window as any).__ALEX_MOCK_STREAM__?.pushEvent)
    );

    await page.evaluate(() => {
      const controls = (window as any).__ALEX_MOCK_STREAM__;
      const markdown = [
        '# Preview Title',
        '',
        '## Section Heading',
        '',
        'Paragraph content for spacing checks.',
      ].join('\n');
      const uri = `data:text/markdown;base64,${btoa(unescape(encodeURIComponent(markdown)))}`;
      controls.pushEvent({
        event_type: 'workflow.result.final',
        final_answer: 'Markdown preview spacing audit.',
        total_iterations: 1,
        total_tokens: 80,
        stop_reason: 'final_answer',
        duration: 500,
        attachments: {
          'preview.md': {
            name: 'preview.md',
            description: 'preview.md',
            media_type: 'text/markdown',
            format: 'md',
            kind: 'artifact',
            uri,
            preview_profile: 'document.markdown',
          },
        },
      });
    });

    const previewCardButton = page.getByRole('button', {
      name: /preview preview\.md/i,
    });
    await expect(previewCardButton).toBeVisible({ timeout: 60000 });
    await previewCardButton.click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const heading = dialog
      .getByRole('heading', { name: 'Section Heading' })
      .first();
    await expect(heading).toBeVisible();

    const headingMetrics = await heading.evaluate((el: HTMLElement) => {
      const style = window.getComputedStyle(el);
      return {
        marginTop: parseFloat(style.marginTop || '0'),
        marginBottom: parseFloat(style.marginBottom || '0'),
      };
    });

    expect(headingMetrics.marginTop).toBeLessThanOrEqual(16);
    expect(headingMetrics.marginBottom).toBeLessThanOrEqual(10);
  });
});
