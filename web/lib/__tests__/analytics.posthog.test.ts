import { beforeEach, describe, expect, it, vi } from 'vitest';

describe('analytics client', () => {
  beforeEach(async () => {
    vi.resetModules();
    process.env.NEXT_PUBLIC_POSTHOG_KEY = '';
    process.env.NEXT_PUBLIC_POSTHOG_HOST = '';
    const posthog = (await import('posthog-js')).default as any;
    posthog.init.mockClear();
    posthog.capture.mockClear();
    posthog.reset.mockClear();
  });

  it('does not initialize when the API key is missing', async () => {
    const { initAnalytics } = await import('../analytics/posthog');
    const posthog = (await import('posthog-js')).default as any;

    initAnalytics();

    expect(posthog.init).not.toHaveBeenCalled();
    expect(posthog.capture).not.toHaveBeenCalled();
  });

  it('queues events until PostHog is initialized', async () => {
    process.env.NEXT_PUBLIC_POSTHOG_KEY = 'test-key';
    const posthog = (await import('posthog-js')).default as any;
    const analytics = await import('../analytics/posthog');

    analytics.captureEvent('task_submitted', { foo: 'bar' });
    expect(posthog.capture).not.toHaveBeenCalled();

    analytics.initAnalytics();

    expect(posthog.init).toHaveBeenCalledWith(
      'test-key',
      expect.objectContaining({ api_host: expect.any(String) })
    );
    expect(posthog.capture).toHaveBeenCalledWith(
      'task_submitted',
      expect.objectContaining({ foo: 'bar', source: 'web_app' })
    );
  });

  it('keeps an explicit source property provided by callers', async () => {
    process.env.NEXT_PUBLIC_POSTHOG_KEY = 'test-key';
    const posthog = (await import('posthog-js')).default as any;
    const { initAnalytics, captureEvent } = await import('../analytics/posthog');

    initAnalytics();
    posthog.capture.mockClear();

    captureEvent('custom_event', { source: 'manual', value: 1 });

    expect(posthog.capture).toHaveBeenCalledWith('custom_event', {
      source: 'manual',
      value: 1,
    });
  });
});
