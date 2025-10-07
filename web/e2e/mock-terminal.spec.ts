import { test, expect } from '@playwright/test';

test.describe('Mock Manus terminal', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:3000/?mockSSE=1');
  });

  test('streams mock events and renders user task', async ({ page }) => {
    await page.getByTestId('task-input').fill('Refactor the Manus-inspired UI flow');
    await page.getByTestId('task-submit').click();

    await expect(page.getByTestId('event-line-user_task')).toContainText('Refactor the Manus-inspired UI flow');

    await expect(page.getByTestId('event-line-research_plan')).toContainText('Research plan', {
      timeout: 10000,
    });

    await expect(page.getByTestId('event-line-tool_stream_combined')).toContainText('Detected conditional input', {
      timeout: 10000,
    });

    await expect(page.getByTestId('event-line-task_complete')).toContainText('Task Complete', {
      timeout: 10000,
    });
  });

  test('supports reconnection controls through the mock stream API', async ({ page }) => {
    await page.getByTestId('task-input').fill('Simulate reconnection');
    await page.getByTestId('task-submit').click();

    await page.evaluate(() => {
      (window as unknown as { __ALEX_MOCK_STREAM__?: { dropConnection?: () => void } }).__ALEX_MOCK_STREAM__?.dropConnection?.();
    });

    await expect(page.locator('[data-testid="connection-status"]')).toContainText(/reconnect/i, {
      timeout: 5000,
    });

    await page.evaluate(() => {
      (window as unknown as { __ALEX_MOCK_STREAM__?: { restart?: () => void } }).__ALEX_MOCK_STREAM__?.restart?.();
    });

    await expect(page.getByTestId('event-line-tool_stream_combined')).toBeVisible({ timeout: 10000 });
  });

  test('event filters allow focusing on specific event categories', async ({ page }) => {
    await page.getByTestId('task-input').fill('Inspect event filtering');
    await page.getByTestId('task-submit').click();

    const toolsFilter = page.getByTestId('event-filter-tools');
    await expect(toolsFilter).toHaveAttribute('aria-pressed', 'true');

    const combinedStreamEvent = page.getByTestId('event-line-tool_stream_combined');
    await expect(combinedStreamEvent).toBeVisible({ timeout: 10000 });

    await toolsFilter.click();

    await expect(toolsFilter).toHaveAttribute('aria-pressed', 'false');
    await expect(page.getByTestId('event-count-hidden')).toContainText('hidden');
    await expect(page.getByTestId('event-line-tool_stream_combined')).toHaveCount(0);

    await toolsFilter.click();

    await expect(toolsFilter).toHaveAttribute('aria-pressed', 'true');
    await expect(page.getByTestId('event-line-tool_stream_combined')).toBeVisible({ timeout: 10000 });
  });

  test('clear button resets mock session state and removes streamed events', async ({ page }) => {
    await page.getByTestId('task-input').fill('Start and then clear session');
    await page.getByTestId('task-submit').click();

    await expect(page.getByTestId('event-line-user_task')).toBeVisible({ timeout: 5000 });

    const clearButton = page.getByRole('button', { name: /clear session/i });
    await expect(clearButton).toBeVisible({ timeout: 5000 });
    await clearButton.click();

    await expect(page.getByTestId('terminal-output')).toHaveCount(0);
    await expect(page.getByText('No events yet')).toBeVisible();
  });

  test('displays formatted tool call details in Manus style cards', async ({ page }) => {
    await page.getByTestId('task-input').fill('Inspect tool call formatting');
    await page.getByTestId('task-submit').click();

    await expect(page.getByTestId('event-line-tool_call_start')).toBeVisible({ timeout: 5000 });

    await expect(page.getByTestId('tool-call-arguments-mock-call-1')).toContainText('web/app/page.tsx', {
      timeout: 10000,
    });

    await expect(page.getByTestId('tool-call-stream-mock-call-1')).toContainText('Detected conditional input', {
      timeout: 10000,
    });

    await expect(page.getByTestId('tool-call-result-mock-call-1')).toContainText('Successfully reviewed', {
      timeout: 10000,
    });
  });
});
