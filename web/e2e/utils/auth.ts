import type { Page } from '@playwright/test';

const AUTH_STORAGE_KEY = 'alex.console.auth';

interface MockAuthSession {
  accessToken: string;
  accessExpiry: string;
  refreshExpiry: string;
  user: {
    id: string;
    email: string;
    displayName: string;
    pointsBalance: number;
    subscription: {
      tier: string;
      monthlyPriceCents: number;
      expiresAt: string;
      isPaid: boolean;
    };
  };
}

function createExpiry(minutesFromNow: number): string {
  return new Date(Date.now() + minutesFromNow * 60 * 1000).toISOString();
}

export function buildMockAuthSession(): MockAuthSession {
  return {
    accessToken: 'e2e-access-token',
    accessExpiry: createExpiry(30),
    refreshExpiry: createExpiry(120),
    user: {
      id: 'user-e2e',
      email: 'e2e@example.com',
      displayName: 'E2E Tester',
      pointsBalance: 0,
      subscription: {
        tier: 'supporter',
        monthlyPriceCents: 1200,
        expiresAt: createExpiry(240),
        isPaid: true,
      },
    },
  };
}

export async function primeAuthSession(page: Page): Promise<void> {
  const session = buildMockAuthSession();
  await page.addInitScript(
    ({ key, value }) => {
      window.localStorage.setItem(key, value);
    },
    { key: AUTH_STORAGE_KEY, value: JSON.stringify(session) }
  );
}

export { AUTH_STORAGE_KEY };
