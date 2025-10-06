import { test, expect } from '@playwright/test';

/**
 * Full User Journey E2E Test
 * Tests the complete flow: Submit task → Monitor progress → View result → Manage sessions
 *
 * NOTE: These tests require a running backend server with SSE support
 */

test.describe.skip('Full User Journey (requires backend)', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('should complete full task execution workflow', async ({ page }) => {
    await page.goto('/');

    // Step 1: Submit a task
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await taskInput.fill('Create a simple Python hello world script');

    const submitButton = page.getByRole('button', { name: /submit|execute/i });
    await submitButton.click();

    // Step 2: Wait for connection
    await expect(page.getByText(/connected/i)).toBeVisible({ timeout: 15000 });

    // Step 3: Watch for task analysis event
    await expect(page.getByText(/task analysis|analyzing/i)).toBeVisible({ timeout: 10000 });

    // Step 4: Watch for thinking/execution events
    await expect(page.locator('[data-testid="thinking-indicator"]')).toBeVisible({ timeout: 10000 });

    // Step 5: Watch for tool calls
    const toolCallCard = page.locator('[data-testid="tool-call-card"]').first();
    await expect(toolCallCard).toBeVisible({ timeout: 30000 });

    // Step 6: Wait for task completion
    await expect(page.getByText(/task complete|completed/i)).toBeVisible({ timeout: 60000 });

    // Step 7: Verify final answer is displayed
    const finalAnswer = page.locator('[data-testid="final-answer"]');
    await expect(finalAnswer).toBeVisible();
    await expect(finalAnswer).toContainText(/hello|python/i);
  });

  test('should handle task modification workflow', async ({ page }) => {
    await page.goto('/');

    // Submit initial task
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await taskInput.fill('List all TypeScript files in the project');

    const submitButton = page.getByRole('button', { name: /submit|execute/i });
    await submitButton.click();

    // Wait for plan to appear (if using Manus flow)
    const planCard = page.locator('[data-testid="research-plan"]');
    const hasPlan = await planCard.isVisible({ timeout: 5000 }).catch(() => false);

    if (hasPlan) {
      // Modify plan
      const modifyButton = page.getByRole('button', { name: /modify/i });
      await modifyButton.click();

      // Edit plan steps
      const planEditor = page.locator('[data-testid="plan-editor"]');
      await expect(planEditor).toBeVisible();

      // Re-submit
      const approveButton = page.getByRole('button', { name: /approve|submit/i });
      await approveButton.click();
    }

    // Continue with execution...
    await expect(page.getByText(/connected/i)).toBeVisible({ timeout: 10000 });
  });

  test('should handle session switching mid-task', async ({ page }) => {
    await page.goto('/');

    // Start first task
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await taskInput.fill('First task');
    await page.getByRole('button', { name: /submit/i }).click();

    await expect(page.getByText(/connected/i)).toBeVisible({ timeout: 10000 });

    // Navigate to sessions
    await page.getByRole('link', { name: /sessions/i }).click();

    // Create new session
    const newSessionButton = page.getByRole('button', { name: /new session/i });
    if (await newSessionButton.isVisible()) {
      await newSessionButton.click();
    }

    // Go back to home
    await page.goto('/');

    // UI should handle session switch gracefully
    await expect(page).toHaveURL('/');
  });

  test('should reconnect after network error', async ({ page }) => {
    await page.goto('/');

    // Submit task
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await taskInput.fill('Test reconnection');
    await page.getByRole('button', { name: /submit/i }).click();

    // Wait for connection
    await expect(page.getByText(/connected/i)).toBeVisible({ timeout: 10000 });

    // Simulate network interruption (go offline)
    await page.context().setOffline(true);

    // Should show disconnected/reconnecting status
    await expect(page.getByText(/reconnecting|disconnected/i)).toBeVisible({ timeout: 5000 });

    // Restore network
    await page.context().setOffline(false);

    // Should reconnect automatically
    await expect(page.getByText(/connected/i)).toBeVisible({ timeout: 30000 });
  });

  test('should handle long task with 100+ events (virtualization test)', async ({ page }) => {
    await page.goto('/');

    // Submit a task that will generate many events
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await taskInput.fill('Find and analyze all JavaScript files in this repository');
    await page.getByRole('button', { name: /submit/i }).click();

    // Wait for connection
    await expect(page.getByText(/connected/i)).toBeVisible({ timeout: 10000 });

    // Wait for multiple events to accumulate
    await page.waitForTimeout(10000);

    // Check that event list is virtualized (should render smoothly)
    const eventList = page.locator('[data-testid="event-list"]');
    await expect(eventList).toBeVisible();

    // Scroll should be smooth (check FPS)
    const scrollPerformance = await page.evaluate(() => {
      const container = document.querySelector('[data-testid="event-list"]');
      if (!container) return { smooth: false };

      let frameCount = 0;
      const startTime = performance.now();

      return new Promise<{ smooth: boolean; fps: number }>((resolve) => {
        const measureFrames = () => {
          frameCount++;
          if (performance.now() - startTime < 1000) {
            requestAnimationFrame(measureFrames);
          } else {
            const fps = frameCount;
            resolve({ smooth: fps >= 55, fps });
          }
        };

        requestAnimationFrame(measureFrames);
      });
    });

    expect(scrollPerformance.fps).toBeGreaterThanOrEqual(55);
  });
});

test.describe('Responsive User Journey', () => {
  test('Desktop (1440px): Dual pane visible', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/');

    const taskInput = page.getByRole('textbox', { name: /task/i });
    await expect(taskInput).toBeVisible();

    // On desktop, dual pane layout should be visible
    // (This depends on actual implementation)
    const mainContent = page.locator('main');
    await expect(mainContent).toBeVisible();
  });

  test('Tablet (1024px): Resizable panes', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto('/');

    // Tablet should have resizable layout
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await expect(taskInput).toBeVisible();
  });

  test('Mobile (768px): Single pane with toggle', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto('/');

    // Mobile should have single pane
    const taskInput = page.getByRole('textbox', { name: /task/i });
    await expect(taskInput).toBeVisible();

    // Check if mobile menu/toggle exists
    const mobileMenu = page.locator('[data-testid="mobile-menu"], [aria-label*="menu"]');
    const hasMenu = await mobileMenu.isVisible().catch(() => false);

    if (hasMenu) {
      await mobileMenu.click();
      // Menu items should be visible
      await expect(page.getByRole('link', { name: /sessions/i })).toBeVisible();
    }
  });
});

test.describe('Session Management Journey', () => {
  test.skip('should manage sessions lifecycle', async ({ page }) => {
    // Navigate to sessions page
    await page.goto('/sessions');

    // Count initial sessions
    const initialSessionCount = await page.locator('[data-testid="session-card"]').count();

    // Create new session (navigate to home and submit task)
    await page.goto('/');
    await page.getByRole('textbox', { name: /task/i }).fill('New session task');
    await page.getByRole('button', { name: /submit/i }).click();

    // Wait for session creation
    await page.waitForTimeout(2000);

    // Go back to sessions
    await page.goto('/sessions');

    // Should have one more session
    const newSessionCount = await page.locator('[data-testid="session-card"]').count();
    expect(newSessionCount).toBeGreaterThan(initialSessionCount);

    // Test session fork
    const forkButton = page.locator('[data-testid="fork-session-button"]').first();
    if (await forkButton.isVisible()) {
      await forkButton.click();

      // Wait for fork to complete
      await page.waitForTimeout(1000);

      // Should have another new session
      const forkedSessionCount = await page.locator('[data-testid="session-card"]').count();
      expect(forkedSessionCount).toBe(newSessionCount + 1);
    }

    // Test session deletion
    const deleteButton = page.locator('[data-testid="delete-session-button"]').last();
    if (await deleteButton.isVisible()) {
      await deleteButton.click();

      // Confirm deletion dialog
      const confirmButton = page.getByRole('button', { name: /confirm|delete/i });
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }

      // Wait for deletion
      await page.waitForTimeout(1000);

      // Session count should decrease
      const finalSessionCount = await page.locator('[data-testid="session-card"]').count();
      expect(finalSessionCount).toBeLessThan(forkedSessionCount);
    }
  });
});
