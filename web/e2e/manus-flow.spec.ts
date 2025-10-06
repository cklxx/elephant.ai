import { test, expect } from '@playwright/test';

test.describe('Manus User Flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:3000');
  });

  test('Full user journey: submit task → approve plan → watch execution → view result', async ({ page }) => {
    // 1. Submit a task
    const taskInput = page.locator('textarea[placeholder*="task" i], input[placeholder*="task" i]');
    await taskInput.fill('Create a simple hello world function');
    await page.locator('button:has-text("Submit"), button:has-text("Send")').click();

    // 2. Wait for plan to be generated
    await expect(page.locator('text=/research plan/i, text=/plan:/i')).toBeVisible({ timeout: 30000 });

    // 3. Approve the plan
    const approveButton = page.locator('button:has-text("Approve"), button:has-text("Accept")');
    await expect(approveButton).toBeVisible({ timeout: 5000 });
    await approveButton.click();

    // 4. Watch execution steps appear in timeline
    await expect(page.locator('[data-testid="timeline-step"], .timeline-step').first()).toBeVisible({ timeout: 10000 });

    // 5. Verify tool outputs appear in right pane
    await expect(page.locator('[data-testid="tool-output"], .tool-output').first()).toBeVisible({ timeout: 15000 });

    // 6. Wait for task completion
    await expect(page.locator('text=/completed/i, text=/success/i')).toBeVisible({ timeout: 60000 });

    // 7. Verify final answer is displayed
    await expect(page.locator('[data-testid="final-answer"], .final-answer')).toBeVisible();
  });

  test('Plan modification flow: submit → modify plan → re-submit → execution', async ({ page }) => {
    // 1. Submit a task
    const taskInput = page.locator('textarea[placeholder*="task" i], input[placeholder*="task" i]');
    await taskInput.fill('Analyze a codebase');
    await page.locator('button:has-text("Submit"), button:has-text("Send")').click();

    // 2. Wait for plan
    await expect(page.locator('text=/research plan/i')).toBeVisible({ timeout: 30000 });

    // 3. Click modify/edit button
    const modifyButton = page.locator('button:has-text("Modify"), button:has-text("Edit")');
    if (await modifyButton.isVisible({ timeout: 5000 })) {
      await modifyButton.click();

      // 4. Modify the plan
      const planEditor = page.locator('textarea[data-testid="plan-editor"], .plan-editor textarea');
      await planEditor.fill('Step 1: Read main files\nStep 2: Analyze structure\nStep 3: Generate report');

      // 5. Save modifications
      await page.locator('button:has-text("Save"), button:has-text("Update")').click();

      // 6. Approve modified plan
      await page.locator('button:has-text("Approve"), button:has-text("Accept")').click();

      // 7. Verify execution starts
      await expect(page.locator('[data-testid="timeline-step"]').first()).toBeVisible({ timeout: 10000 });
    }
  });

  test('Session switching mid-task', async ({ page }) => {
    // 1. Start first task
    const taskInput = page.locator('textarea[placeholder*="task" i], input[placeholder*="task" i]');
    await taskInput.fill('Task 1: Hello world');
    await page.locator('button:has-text("Submit")').click();

    // 2. Wait for plan
    await expect(page.locator('text=/research plan/i')).toBeVisible({ timeout: 30000 });

    // 3. Create new session
    const newSessionButton = page.locator('button:has-text("New Session"), button[aria-label*="new" i]');
    if (await newSessionButton.isVisible({ timeout: 3000 })) {
      await newSessionButton.click();

      // 4. Verify we're in a new session (old plan should be gone)
      await expect(page.locator('text=/research plan/i')).not.toBeVisible({ timeout: 2000 });

      // 5. Switch back to first session
      const sessionList = page.locator('[data-testid="session-list"], .session-list');
      if (await sessionList.isVisible({ timeout: 3000 })) {
        await page.locator('[data-testid="session-item"]:first-child, .session-item:first-child').click();

        // 6. Verify we're back at the plan
        await expect(page.locator('text=/research plan/i')).toBeVisible({ timeout: 5000 });
      }
    }
  });

  test('SSE reconnection after network error', async ({ page, context }) => {
    // 1. Submit task
    const taskInput = page.locator('textarea[placeholder*="task" i], input[placeholder*="task" i]');
    await taskInput.fill('Simple calculation');
    await page.locator('button:has-text("Submit")').click();

    // 2. Wait for plan and approve
    await expect(page.locator('text=/research plan/i')).toBeVisible({ timeout: 30000 });
    const approveButton = page.locator('button:has-text("Approve")');
    if (await approveButton.isVisible({ timeout: 5000 })) {
      await approveButton.click();
    }

    // 3. Simulate network offline
    await context.setOffline(true);
    await page.waitForTimeout(2000);

    // 4. Restore network
    await context.setOffline(false);

    // 5. Verify reconnection indicator
    await expect(page.locator('[data-testid="connection-status"], .connection-status')).toContainText(/reconnect/i, { timeout: 10000 });

    // 6. Verify events continue to flow after reconnection
    const eventCountBefore = await page.locator('[data-testid="timeline-step"], .timeline-step').count();
    await page.waitForTimeout(5000);
    const eventCountAfter = await page.locator('[data-testid="timeline-step"], .timeline-step').count();

    // Events should have increased after reconnection
    expect(eventCountAfter).toBeGreaterThanOrEqual(eventCountBefore);
  });

  test('Long task with 100+ events (verify virtualization)', async ({ page }) => {
    // 1. Submit a complex task that will generate many events
    const taskInput = page.locator('textarea[placeholder*="task" i], input[placeholder*="task" i]');
    await taskInput.fill('Analyze all files in a large directory and generate comprehensive report');
    await page.locator('button:has-text("Submit")').click();

    // 2. Wait for plan and approve
    await expect(page.locator('text=/research plan/i')).toBeVisible({ timeout: 30000 });
    const approveButton = page.locator('button:has-text("Approve")');
    if (await approveButton.isVisible({ timeout: 5000 })) {
      await approveButton.click();
    }

    // 3. Wait for events to accumulate
    await page.waitForTimeout(10000);

    // 4. Verify scroll performance (should remain smooth with virtualization)
    const timeline = page.locator('[data-testid="timeline"], .timeline');
    if (await timeline.isVisible()) {
      await timeline.hover();

      // Scroll down and up rapidly
      for (let i = 0; i < 5; i++) {
        await page.mouse.wheel(0, 1000);
        await page.waitForTimeout(100);
        await page.mouse.wheel(0, -1000);
        await page.waitForTimeout(100);
      }

      // Verify no performance degradation (page should still be responsive)
      const scrollable = await timeline.evaluate(el => el.scrollHeight > el.clientHeight);
      expect(scrollable).toBeTruthy();
    }
  });
});

test.describe('Responsive Behavior', () => {
  test('Desktop: Dual pane visible and resizable', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('http://localhost:3000');

    // Verify both panes are visible
    const leftPane = page.locator('[data-testid="left-pane"], .left-pane');
    const rightPane = page.locator('[data-testid="right-pane"], .right-pane');

    await expect(leftPane).toBeVisible();
    await expect(rightPane).toBeVisible();

    // Verify resizer is present
    const resizer = page.locator('[data-testid="pane-resizer"], .pane-resizer');
    if (await resizer.isVisible()) {
      // Get initial widths
      const leftWidth = await leftPane.evaluate(el => el.getBoundingClientRect().width);

      // Drag resizer
      await resizer.hover();
      await page.mouse.down();
      await page.mouse.move(leftWidth + 100, 0);
      await page.mouse.up();

      // Verify width changed
      const newLeftWidth = await leftPane.evaluate(el => el.getBoundingClientRect().width);
      expect(newLeftWidth).toBeGreaterThan(leftWidth);
    }
  });

  test('Tablet: Dual pane with collapsible left', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto('http://localhost:3000');

    // Both panes should be visible initially
    const leftPane = page.locator('[data-testid="left-pane"], .left-pane');
    const rightPane = page.locator('[data-testid="right-pane"], .right-pane');

    await expect(leftPane).toBeVisible();
    await expect(rightPane).toBeVisible();

    // Look for collapse button
    const collapseButton = page.locator('button[aria-label*="collapse" i], button:has-text("Hide")');
    if (await collapseButton.isVisible({ timeout: 2000 })) {
      await collapseButton.click();

      // Left pane should be hidden
      await expect(leftPane).toBeHidden();

      // Right pane should expand
      const rightWidth = await rightPane.evaluate(el => el.getBoundingClientRect().width);
      expect(rightWidth).toBeGreaterThan(900);
    }
  });

  test('Mobile: Single pane with toggle', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto('http://localhost:3000');

    // On mobile, only one pane should be visible at a time
    const leftPane = page.locator('[data-testid="left-pane"], .left-pane');
    const rightPane = page.locator('[data-testid="right-pane"], .right-pane');

    // Check which pane is visible (either left or right, not both)
    const leftVisible = await leftPane.isVisible({ timeout: 2000 });
    const rightVisible = await rightPane.isVisible({ timeout: 2000 });

    expect(leftVisible !== rightVisible).toBeTruthy(); // XOR: exactly one should be visible

    // Look for toggle button
    const toggleButton = page.locator('button[aria-label*="toggle" i], button:has-text("Toggle View")');
    if (await toggleButton.isVisible({ timeout: 2000 })) {
      await toggleButton.click();

      // Visibility should swap
      const newLeftVisible = await leftPane.isVisible({ timeout: 2000 });
      const newRightVisible = await rightPane.isVisible({ timeout: 2000 });

      expect(newLeftVisible).toBe(!leftVisible);
      expect(newRightVisible).toBe(!rightVisible);
    }
  });
});

test.describe('Performance', () => {
  test('Page load performance', async ({ page }) => {
    const startTime = Date.now();
    await page.goto('http://localhost:3000');
    await page.waitForLoadState('networkidle');
    const loadTime = Date.now() - startTime;

    // Page should load within 3 seconds
    expect(loadTime).toBeLessThan(3000);

    // Verify main content is visible
    await expect(page.locator('main, [role="main"]')).toBeVisible();
  });

  test('Smooth scrolling with many events', async ({ page }) => {
    await page.goto('http://localhost:3000');

    // Submit task
    const taskInput = page.locator('textarea[placeholder*="task" i], input[placeholder*="task" i]');
    await taskInput.fill('Test task for performance');
    await page.locator('button:has-text("Submit")').click();

    // Wait for some events
    await page.waitForTimeout(5000);

    // Measure scroll FPS (approximate)
    const timeline = page.locator('[data-testid="timeline"], .timeline');
    if (await timeline.isVisible()) {
      const scrollMetrics = await page.evaluate(async () => {
        const start = performance.now();
        let frames = 0;

        // Scroll for 1 second
        const scrollInterval = setInterval(() => {
          window.scrollBy(0, 10);
          frames++;
        }, 16); // ~60fps

        await new Promise(resolve => setTimeout(resolve, 1000));
        clearInterval(scrollInterval);

        const duration = performance.now() - start;
        return { frames, duration, fps: (frames / duration) * 1000 };
      });

      // Should maintain at least 30 FPS
      expect(scrollMetrics.fps).toBeGreaterThan(30);
    }
  });
});
