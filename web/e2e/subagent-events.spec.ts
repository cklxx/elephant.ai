import { expect, test } from '@playwright/test';
import { primeAuthSession } from './utils/auth';
import {
  capturePageScreenshot,
  shouldCaptureScreenshots,
} from './utils/screenshots';

const STORAGE_KEY = 'alex-session-storage';

const clearStorage = () => {
  window.localStorage.clear();
};

test.describe('Subagent event rendering', () => {
  test('renders mock subagent tool output and headers', async ({ page }) => {
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
    await textarea.fill('Review nested tool call output for subagents.');
    await textarea.press('Enter');

    const firstSubagentTimeline = page
      .getByTestId('event-subagent-iteration_start')
      .first();
    await expect(firstSubagentTimeline).toBeVisible({ timeout: 30000 });
    await expect(
      firstSubagentTimeline.getByText('Research comparable console UX patterns')
    ).toBeVisible();
    await expect(firstSubagentTimeline.getByText('Parallel ×2')).toBeVisible();

    const secondSubagentTimeline = page
      .getByTestId('event-subagent-iteration_start')
      .nth(1);
    await expect(secondSubagentTimeline).toBeVisible();
    await expect(
      secondSubagentTimeline.getByText(
        'Inspect tool output rendering implementation'
      )
    ).toBeVisible();

    const subagentToolStreams = page.getByTestId('event-subagent-tool_call_stream');
    await expect(subagentToolStreams.nth(0)).toContainText(
      'Summarizing multi-panel timelines'
    );
    await expect(subagentToolStreams.nth(2)).toContainText(
      'Traced ToolOutputCard props'
    );

    const subagentToolCompletions = page.getByTestId(
      'event-subagent-tool_call_complete'
    );
    await expect(subagentToolCompletions.first()).toContainText('web_search');
    await expect(subagentToolCompletions.nth(1)).toContainText('code_search');

    const subagentCompletions = page.getByTestId('event-subagent-task_complete');
    await expect(subagentCompletions.first()).toContainText('Validated layout guidance');
    await expect(subagentCompletions.nth(1)).toContainText('Confirmed ToolOutputCard handles metadata');

    if (shouldCaptureScreenshots) {
      await capturePageScreenshot(page, 'subagent-events');
    }
  });
});
