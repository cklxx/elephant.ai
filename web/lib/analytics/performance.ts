/**
 * Performance Monitoring Utilities
 * Provides infrastructure for tracking custom performance metrics
 * Ready for Sentry integration
 */

export interface PerformanceMetric {
  name: string;
  value: number;
  unit: 'millisecond' | 'byte' | 'count';
  context?: Record<string, unknown>;
  timestamp: number;
}

export interface TTFTMetric {
  sessionId: string;
  duration: number;
  timestamp: number;
}

export interface SSEConnectionMetric {
  sessionId: string;
  duration: number;
  success: boolean;
  reconnectCount: number;
  timestamp: number;
}

/**
 * Performance metrics collector
 * Currently logs to console, ready for Sentry integration
 */
class PerformanceMonitor {
  private metrics: PerformanceMetric[] = [];
  private readonly maxMetrics = 100;

  /**
   * Track a custom performance metric
   */
  track(metric: Omit<PerformanceMetric, 'timestamp'>): void {
    const fullMetric: PerformanceMetric = {
      ...metric,
      timestamp: Date.now(),
    };

    this.metrics.push(fullMetric);

    // Keep only recent metrics
    if (this.metrics.length > this.maxMetrics) {
      this.metrics = this.metrics.slice(-this.maxMetrics);
    }

    // Log in development
    if (process.env.NODE_ENV === 'development') {
      console.log('[Performance]', metric.name, `${metric.value}${metric.unit}`);
    }

    // TODO: Send to Sentry when integrated
    // Sentry.setMeasurement(metric.name, metric.value, metric.unit);
  }

  /**
   * Track Time To First Token (TTFT)
   */
  trackTTFT(sessionId: string, duration: number): void {
    this.track({
      name: 'ttft',
      value: duration,
      unit: 'millisecond',
      context: { sessionId },
    });
  }

  /**
   * Track SSE connection metrics
   */
  trackSSEConnection(data: Omit<SSEConnectionMetric, 'timestamp'>): void {
    this.track({
      name: 'sse_connection',
      value: data.duration,
      unit: 'millisecond',
      context: {
        sessionId: data.sessionId,
        success: data.success,
        reconnectCount: data.reconnectCount,
      },
    });
  }

  /**
   * Track rendering performance
   */
  trackRenderTime(component: string, duration: number): void {
    this.track({
      name: 'render_time',
      value: duration,
      unit: 'millisecond',
      context: { component },
    });
  }

  /**
   * Track memory usage
   */
  trackMemoryUsage(context?: Record<string, unknown>): void {
    if ('memory' in performance) {
      const memory = (performance as any).memory;
      this.track({
        name: 'memory_usage',
        value: memory.usedJSHeapSize,
        unit: 'byte',
        context: {
          ...context,
          totalHeapSize: memory.totalJSHeapSize,
          heapLimit: memory.jsHeapSizeLimit,
        },
      });
    }
  }

  /**
   * Get all collected metrics
   */
  getMetrics(): PerformanceMetric[] {
    return [...this.metrics];
  }

  /**
   * Clear all metrics
   */
  clear(): void {
    this.metrics = [];
  }
}

// Export singleton instance
export const performanceMonitor = new PerformanceMonitor();

/**
 * React hook-friendly wrapper
 */
export function usePerformanceTracking() {
  return {
    trackTTFT: (sessionId: string, duration: number) =>
      performanceMonitor.trackTTFT(sessionId, duration),
    trackSSE: (data: Omit<SSEConnectionMetric, 'timestamp'>) =>
      performanceMonitor.trackSSEConnection(data),
    trackRender: (component: string, duration: number) =>
      performanceMonitor.trackRenderTime(component, duration),
    trackMemory: (context?: Record<string, unknown>) =>
      performanceMonitor.trackMemoryUsage(context),
  };
}
