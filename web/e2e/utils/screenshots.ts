import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import type { Page } from '@playwright/test';

const SCREENSHOT_ENV_FLAG = 'CAPTURE_E2E_SCREENSHOTS';
const SCREENSHOT_DIR = 'test-results/screenshots';

export const shouldCaptureScreenshots =
  process.env[SCREENSHOT_ENV_FLAG] === '1' ||
  process.env[SCREENSHOT_ENV_FLAG]?.toLowerCase() === 'true';

export async function capturePageScreenshot(
  page: Page,
  name: string
): Promise<string> {
  mkdirSync(SCREENSHOT_DIR, { recursive: true });
  const path = join(SCREENSHOT_DIR, `${name}.png`);
  await page.screenshot({ path, fullPage: true });
  return path;
}
