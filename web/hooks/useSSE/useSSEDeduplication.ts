/**
 * Hook for SSE event deduplication.
 * Maintains a sliding window cache of seen event signatures to prevent
 * duplicate event processing.
 */

import { useCallback, useRef } from "react";
import type { AnyAgentEvent } from "@/lib/types";
import { buildEventSignature } from "@/lib/events/signature";
const MAX_DEDUPE_CACHE_SIZE = 2000;

/**
 * Ring-buffer backed dedup cache.
 * Avoids O(n) Array.shift() by using a fixed-size circular buffer with a
 * head pointer. Eviction is O(1).
 */
class RingDedupeCache {
  private seen = new Set<string>();
  private ring: (string | null)[];
  private head = 0;
  private count = 0;
  private readonly capacity: number;

  constructor(capacity: number) {
    this.capacity = capacity;
    this.ring = new Array<string | null>(capacity).fill(null);
  }

  has(key: string): boolean {
    return this.seen.has(key);
  }

  add(key: string): void {
    if (this.count >= this.capacity) {
      // Evict oldest entry at head – O(1)
      const oldest = this.ring[this.head];
      if (oldest !== null) {
        this.seen.delete(oldest);
      }
    } else {
      this.count++;
    }

    this.ring[this.head] = key;
    this.head = (this.head + 1) % this.capacity;
    this.seen.add(key);
  }

  get size(): number {
    return this.seen.size;
  }

  reset(): void {
    this.seen.clear();
    this.ring.fill(null);
    this.head = 0;
    this.count = 0;
  }
}

export interface UseSSEDeduplicationReturn {
  /** Check if event has been seen before and mark it as seen if not */
  dedupeEvent: (event: AnyAgentEvent) => boolean;
  /** Reset the deduplication cache */
  resetDedupe: () => void;
  /** Get current cache for debugging */
  getCacheSize: () => number;
}

export function useSSEDeduplication(): UseSSEDeduplicationReturn {
  const cacheRef = useRef<RingDedupeCache>(new RingDedupeCache(MAX_DEDUPE_CACHE_SIZE));

  const resetDedupe = useCallback(() => {
    cacheRef.current.reset();
  }, []);

  const dedupeEvent = useCallback((event: AnyAgentEvent): boolean => {
    const dedupeKey = buildEventSignature(event);
    const cache = cacheRef.current;

    if (cache.has(dedupeKey)) {
      return true; // Event is a duplicate
    }

    cache.add(dedupeKey);
    return false; // Event is new
  }, []);

  const getCacheSize = useCallback(() => {
    return cacheRef.current.size;
  }, []);

  return {
    dedupeEvent,
    resetDedupe,
    getCacheSize,
  };
}
