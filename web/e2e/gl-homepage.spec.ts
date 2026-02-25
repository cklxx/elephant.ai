import { test, expect } from '@playwright/test';

test.describe('GL Homepage', () => {
  test('renders title and CTA on root path', async ({ page }) => {
    await page.goto('/');

    // Title should be visible
    const title = page.locator('h1');
    await expect(title).toBeVisible({ timeout: 10000 });
    await expect(title).toHaveText('elephant.ai');

    // CTA link should be present and point to /conversation
    const cta = page.locator('a[href="/conversation"]');
    await expect(cta).toBeVisible();
    await expect(cta).toContainText('Console');
  });

  test('renders Chinese copy on /zh path', async ({ page }) => {
    await page.goto('/zh');

    const title = page.locator('h1');
    await expect(title).toBeVisible({ timeout: 10000 });
    await expect(title).toHaveText('elephant.ai');

    // CTA should show Chinese text
    const cta = page.locator('a[href="/conversation"]');
    await expect(cta).toBeVisible();
    await expect(cta).toContainText('控制台');
  });

  test('language toggle links are present', async ({ page }) => {
    await page.goto('/');

    // Should have links to both languages
    const zhLink = page.locator('a[href="/zh"]');
    const enLink = page.locator('a[href="/"]');
    await expect(zhLink).toBeVisible({ timeout: 10000 });
    await expect(enLink).toBeVisible();
  });

  test('canvas element loads when WebGL is available', async ({ page }) => {
    await page.goto('/');

    // R3F renders a <canvas> element
    const canvas = page.locator('canvas');
    // Wait for the dynamic import to load
    await expect(canvas).toBeVisible({ timeout: 15000 });
  });

  test('page has dark background', async ({ page }) => {
    await page.goto('/');

    // The root container should have the dark background
    const bg = await page.locator('div[style*="#080810"]').first();
    await expect(bg).toBeVisible({ timeout: 10000 });
  });
});
