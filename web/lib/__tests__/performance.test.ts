import { describe, expect, it, vi, beforeEach } from 'vitest';

// Use a fresh import for each test to avoid singleton state leakage
let performanceMonitor: typeof import('@/lib/analytics/performance').performanceMonitor;

beforeEach(async () => {
  const mod = await import('@/lib/analytics/performance');
  performanceMonitor = mod.performanceMonitor;
  performanceMonitor.clear();
});

describe('PerformanceMonitor', () => {
  it('trackTTFT stores metric with correct name and context', () => {
    performanceMonitor.trackTTFT('session-1', 250);

    const metrics = performanceMonitor.getMetrics();
    expect(metrics).toHaveLength(1);
    expect(metrics[0]).toMatchObject({
      name: 'ttft',
      value: 250,
      unit: 'millisecond',
      context: { sessionId: 'session-1' },
    });
    expect(metrics[0].timestamp).toBeGreaterThan(0);
  });

  it('trackSSEConnection stores context fields correctly', () => {
    performanceMonitor.trackSSEConnection({
      sessionId: 'session-2',
      duration: 1500,
      success: true,
      reconnectCount: 2,
    });

    const metrics = performanceMonitor.getMetrics();
    expect(metrics).toHaveLength(1);
    expect(metrics[0]).toMatchObject({
      name: 'sse_connection',
      value: 1500,
      unit: 'millisecond',
      context: {
        sessionId: 'session-2',
        success: true,
        reconnectCount: 2,
      },
    });
  });

  it('trackRenderTime stores component name and duration', () => {
    performanceMonitor.trackRenderTime('ConversationEventStream', 16.5);

    const metrics = performanceMonitor.getMetrics();
    expect(metrics).toHaveLength(1);
    expect(metrics[0]).toMatchObject({
      name: 'render_time',
      value: 16.5,
      unit: 'millisecond',
      context: { component: 'ConversationEventStream' },
    });
  });

  it('evicts oldest entries when exceeding 100 metrics', () => {
    for (let i = 0; i < 110; i++) {
      performanceMonitor.track({
        name: `metric_${i}`,
        value: i,
        unit: 'count',
      });
    }

    const metrics = performanceMonitor.getMetrics();
    expect(metrics).toHaveLength(100);
    // Oldest entries should have been evicted
    expect(metrics[0].name).toBe('metric_10');
    expect(metrics[99].name).toBe('metric_109');
  });

  it('clear() empties all stored metrics', () => {
    performanceMonitor.trackTTFT('s1', 100);
    performanceMonitor.trackTTFT('s2', 200);
    expect(performanceMonitor.getMetrics()).toHaveLength(2);

    performanceMonitor.clear();
    expect(performanceMonitor.getMetrics()).toHaveLength(0);
  });

  it('getMetrics() returns a copy, not the internal array', () => {
    performanceMonitor.trackTTFT('s1', 100);

    const metrics1 = performanceMonitor.getMetrics();
    const metrics2 = performanceMonitor.getMetrics();

    expect(metrics1).not.toBe(metrics2);
    expect(metrics1).toEqual(metrics2);
  });

  it('trackMemoryUsage is a graceful no-op when performance.memory is absent', () => {
    // performance.memory is Chrome-only and not available in happy-dom
    expect(() => performanceMonitor.trackMemoryUsage()).not.toThrow();
    // No metric should be added if memory API is absent
    expect(performanceMonitor.getMetrics()).toHaveLength(0);
  });

  it('does not log to console outside development mode', () => {
    const logMock = vi.mocked(console.log);
    logMock.mockClear();

    performanceMonitor.track({
      name: 'test_metric',
      value: 42,
      unit: 'count',
    });

    // NODE_ENV is 'test', so console.log should NOT be called
    expect(logMock).not.toHaveBeenCalled();
  });

  it('track() adds timestamp automatically', () => {
    const before = Date.now();
    performanceMonitor.track({
      name: 'timed',
      value: 1,
      unit: 'count',
    });
    const after = Date.now();

    const metric = performanceMonitor.getMetrics()[0];
    expect(metric.timestamp).toBeGreaterThanOrEqual(before);
    expect(metric.timestamp).toBeLessThanOrEqual(after);
  });
});
