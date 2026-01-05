import { expect, test } from '@playwright/test';
import { primeAuthSession } from './utils/auth';

const clearStorage = () => {
  window.localStorage.clear();
};

const spacingMarkdown = [
  '# 抖音最火AI账号分析报告',
  '',
  '## 发帖规律',
  '1. 发布时间：结合用户活跃高峰时段，通常在早上8点、午饭后12点、晚上8点三个时间段集中发布。',
  '2. 发布频率：每日 3-5 篇，保持稳定的内容输出节奏，减少单次刷屏。',
  '3. 内容类型：短视频、图文短稿，结合热点话题，提升风格统一度。',
].join('\n');

test.describe('Final summary markdown attachments', () => {
  test('renders compact line spacing for markdown artifact previews', async ({ page }) => {
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
    await textarea.fill('Check markdown spacing in summary.');
    await textarea.press('Enter');

    await expect(page.getByTestId('event-workflow.result.final').first()).toBeVisible({ timeout: 60000 });
    await page.waitForFunction(() => Boolean((window as any).__ALEX_MOCK_STREAM__?.pushEvent));

    const finalAnswer = 'Spacing report ready [SpacingReport]';
    await page.evaluate(
      ({ markdown, answer }) => {
        const controls = (window as any).__ALEX_MOCK_STREAM__;
        if (!controls?.pushEvent) {
          throw new Error('Mock stream controls unavailable');
        }
        const dataUri = `data:text/markdown;charset=utf-8,${encodeURIComponent(markdown)}`;
        controls.pushEvent({
          event_type: 'workflow.result.final',
          agent_level: 'core',
          final_answer: answer,
          total_iterations: 1,
          total_tokens: 0,
          stop_reason: 'final_answer',
          duration: 0,
          is_streaming: false,
          stream_finished: true,
          session_id: 'spacing-session',
          task_id: `spacing-task-${Date.now()}`,
          attachments: {
            SpacingReport: {
              name: 'SpacingReport.md',
              description: 'SpacingReport',
              media_type: 'text/markdown',
              format: 'markdown',
              kind: 'artifact',
              uri: dataUri,
            },
          },
        });
      },
      { markdown: spacingMarkdown, answer: finalAnswer },
    );

    const customSummary = page
      .getByTestId('event-workflow.result.final')
      .filter({ hasText: 'Spacing report ready' })
      .first();
    await expect(customSummary).toBeVisible({ timeout: 15000 });
    await customSummary.scrollIntoViewIfNeeded();

    const reportCard = customSummary
      .getByTestId('artifact-preview-card')
      .filter({ hasText: 'SpacingReport' })
      .first();
    await expect(reportCard).toBeVisible({ timeout: 15000 });
    await expect(reportCard.getByText('发帖规律')).toBeVisible({ timeout: 15000 });

    const lineHeightRatios = await reportCard.locator('.markdown-body li').evaluateAll((nodes) =>
      nodes.map((node) => {
        const style = getComputedStyle(node);
        return parseFloat(style.lineHeight) / parseFloat(style.fontSize);
      }),
    );
    const maxRatio = Math.max(...lineHeightRatios);
    expect(maxRatio).toBeLessThanOrEqual(1.56);
  });
});
