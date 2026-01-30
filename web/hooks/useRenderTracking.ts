import { useEffect, useLayoutEffect, useRef } from "react";
import { performanceMonitor } from "@/lib/analytics/performance";

/**
 * Lightweight hook that tracks component render duration via performanceMonitor.
 * Dev-only: guarded by process.env.NODE_ENV === 'development'.
 *
 * Usage: call at the top of a component body.
 *   useRenderTracking('ConversationEventStream');
 */
export function useRenderTracking(componentName: string): void {
  const renderStartRef = useRef<number>(0);

  useLayoutEffect(() => {
    if (process.env.NODE_ENV !== "development") {
      return;
    }
    renderStartRef.current = performance.now();
  });

  useEffect(() => {
    if (process.env.NODE_ENV !== "development") {
      return;
    }

    const duration = performance.now() - renderStartRef.current;
    performanceMonitor.trackRenderTime(componentName, duration);
  });
}
