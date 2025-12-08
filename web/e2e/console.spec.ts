import { expect, test } from '@playwright/test';

const mockSession = {
  accessToken: 'test-access-token',
  accessExpiry: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
  refreshExpiry: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
  user: {
    id: 'user-1',
    email: 'test@example.com',
    displayName: 'Test User',
    pointsBalance: 0,
    subscription: {
      tier: 'pro',
      monthlyPriceCents: 0,
      expiresAt: null,
      isPaid: false,
    },
    photoURL: null,
  },
};

test.beforeEach(async ({ page }) => {
  await page.addInitScript((session) => {
    window.localStorage.setItem('alex.console.auth', JSON.stringify(session));

    class MockEventSource {
      static instances: MockEventSource[] = [];
      url: string;
      readyState = 1;
      listeners = new Map<string, ((evt: MessageEvent) => void)[]>();

      constructor(url: string) {
        this.url = url;
        (window as any).__eventSources = (window as any).__eventSources || [];
        (window as any).__eventSources.push(this);
        MockEventSource.instances.push(this);
      }

      addEventListener(type: string, handler: (evt: MessageEvent) => void) {
        const current = this.listeners.get(type) ?? [];
        this.listeners.set(type, [...current, handler]);
      }

      emit(type: string, payload: unknown) {
        const listeners = this.listeners.get(type) ?? [];
        const event = new MessageEvent(type, { data: JSON.stringify(payload) });
        listeners.forEach((listener) => listener(event));
      }

      close() {
        this.readyState = 2;
      }
    }

    (window as any).MockEventSource = MockEventSource;
    window.EventSource = MockEventSource as any;
  }, mockSession);

  await page.route('**/api/tasks', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ task_id: 'task-123', session_id: 'session-abc' }),
    });
  });

  await page.route('**/api/sse**', async (route) => {
    await route.fulfill({ status: 200, body: '' });
  });

  await page.route('**/api/sessions', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sessions: [] }),
    });
  });
});

test('launches a task and renders streamed events', async ({ page }) => {
  await page.goto('/');

  await page.getByPlaceholder('Describe the workflow you want ALEX to run...').fill('Investigate logs');
  await page.getByRole('button', { name: 'Launch & stream' }).click();

  await expect(
    page.getByText('Task task-123 created for session session-abc'),
  ).toBeVisible();

  await page.evaluate(() => {
    const source = (window as any).__eventSources?.[0];
    source?.emit?.('workflow.node.completed', { message: 'Node finished' });
  });

  await expect(page.getByText('Node finished')).toBeVisible();
});
