/**
 * Hook for SSE event buffering and flush scheduling.
 * Batches incoming events and flushes them on animation frames
 * or timeouts to optimize rendering performance.
 */

import { useCallback, useEffect, useRef } from "react";
import type { AnyAgentEvent } from "@/lib/types";
import { STREAM_FLUSH_MS, IS_TEST_ENV, type FlushHandle, type FlushMode } from "./types";

/** Flush early when buffer exceeds this threshold to prevent unbounded growth */
const MAX_BUFFER_SIZE = 50;

export interface UseSSEEventBufferOptions {
  onFlush: (events: AnyAgentEvent[]) => void;
}

export interface UseSSEEventBufferReturn {
  /** Enqueue an event for processing */
  enqueueEvent: (event: AnyAgentEvent) => void;
  /** Cancel any scheduled flush */
  cancelScheduledFlush: () => void;
  /** Force an immediate flush */
  flushNow: () => void;
  /** Clear the pending buffer */
  clearBuffer: () => void;
  /** Get the current buffer size */
  getBufferSize: () => number;
}

export function useSSEEventBuffer(
  options: UseSSEEventBufferOptions
): UseSSEEventBufferReturn {
  const { onFlush } = options;

  const pendingEventsRef = useRef<AnyAgentEvent[]>([]);
  const flushHandleRef = useRef<FlushHandle | null>(null);
  const flushModeRef = useRef<FlushMode>(null);
  const onFlushRef = useRef(onFlush);

  // Keep onFlush ref up to date
  useEffect(() => {
    onFlushRef.current = onFlush;
  }, [onFlush]);

  const cancelScheduledFlush = useCallback(() => {
    if (flushHandleRef.current == null) {
      return;
    }

    const handle = flushHandleRef.current;
    const mode = flushModeRef.current;
    flushHandleRef.current = null;
    flushModeRef.current = null;

    if (mode === "raf" && typeof window !== "undefined" && "cancelAnimationFrame" in window) {
      window.cancelAnimationFrame(handle as number);
      return;
    }

    clearTimeout(handle as ReturnType<typeof setTimeout>);
  }, []);

  const flushNow = useCallback(() => {
    const pending = pendingEventsRef.current;
    if (pending.length === 0) {
      return;
    }

    pendingEventsRef.current = [];
    onFlushRef.current(pending);
  }, []);

  const scheduleFlush = useCallback(() => {
    // In test environment, flush synchronously
    if (IS_TEST_ENV) {
      flushNow();
      return;
    }

    // Already scheduled
    if (flushHandleRef.current != null) {
      return;
    }

    // Prefer requestAnimationFrame for smooth rendering
    if (typeof window !== "undefined" && "requestAnimationFrame" in window) {
      flushModeRef.current = "raf";
      flushHandleRef.current = window.requestAnimationFrame(() => {
        flushHandleRef.current = null;
        flushModeRef.current = null;
        flushNow();
      });
      return;
    }

    // Fallback to setTimeout
    flushModeRef.current = "timeout";
    flushHandleRef.current = setTimeout(() => {
      flushHandleRef.current = null;
      flushModeRef.current = null;
      flushNow();
    }, STREAM_FLUSH_MS);
  }, [flushNow]);

  const enqueueEvent = useCallback(
    (event: AnyAgentEvent) => {
      pendingEventsRef.current.push(event);
      // Flush early when buffer is large to prevent unbounded growth
      // (e.g. when RAF is delayed by background tab or GC pause)
      if (pendingEventsRef.current.length >= MAX_BUFFER_SIZE) {
        cancelScheduledFlush();
        flushNow();
      } else {
        scheduleFlush();
      }
    },
    [scheduleFlush, cancelScheduledFlush, flushNow]
  );

  const clearBuffer = useCallback(() => {
    cancelScheduledFlush();
    pendingEventsRef.current = [];
  }, [cancelScheduledFlush]);

  const getBufferSize = useCallback(() => {
    return pendingEventsRef.current.length;
  }, []);

  return {
    enqueueEvent,
    cancelScheduledFlush,
    flushNow,
    clearBuffer,
    getBufferSize,
  };
}
