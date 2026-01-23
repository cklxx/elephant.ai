/**
 * Hook for SSE event deduplication.
 * Maintains a sliding window cache of seen event signatures to prevent
 * duplicate event processing.
 */

import { useCallback, useRef } from "react";
import type { AnyAgentEvent } from "@/lib/types";
import { buildEventSignature } from "@/lib/events/signature";
import type { DedupeCache } from "./types";

const MAX_DEDUPE_CACHE_SIZE = 2000;

export interface UseSSEDeduplicationReturn {
  /** Check if event has been seen before and mark it as seen if not */
  dedupeEvent: (event: AnyAgentEvent) => boolean;
  /** Reset the deduplication cache */
  resetDedupe: () => void;
  /** Get current cache for debugging */
  getCacheSize: () => number;
}

export function useSSEDeduplication(): UseSSEDeduplicationReturn {
  const dedupeRef = useRef<DedupeCache>({
    seen: new Set(),
    order: [],
  });

  const resetDedupe = useCallback(() => {
    dedupeRef.current = {
      seen: new Set(),
      order: [],
    };
  }, []);

  const dedupeEvent = useCallback((event: AnyAgentEvent): boolean => {
    const dedupeKey = buildEventSignature(event);
    const cache = dedupeRef.current;

    if (cache.seen.has(dedupeKey)) {
      return true; // Event is a duplicate
    }

    cache.seen.add(dedupeKey);
    cache.order.push(dedupeKey);

    // Maintain sliding window
    if (cache.order.length > MAX_DEDUPE_CACHE_SIZE) {
      const oldest = cache.order.shift();
      if (oldest) {
        cache.seen.delete(oldest);
      }
    }

    return false; // Event is new
  }, []);

  const getCacheSize = useCallback(() => {
    return dedupeRef.current.seen.size;
  }, []);

  return {
    dedupeEvent,
    resetDedupe,
    getCacheSize,
  };
}
