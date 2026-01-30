/**
 * SSE Hook Family
 *
 * A modular set of hooks for managing SSE connections and event processing.
 *
 * Primary hook:
 * - useSSE: Complete SSE connection management
 *
 * Specialized hooks (can be used independently):
 * - useSSEConnection: Connection lifecycle management
 * - useSSEEventBuffer: Event buffering and flush scheduling
 * - useSSEDeduplication: Event deduplication
 * - useStreamingAnswerBuffer: Streaming answer accumulation
 */

// Main hook
export { useSSE } from "./useSSE";
export type { UseSSEOptions, UseSSEReturn } from "./types";

// Specialized hooks
export { useSSEConnection } from "./useSSEConnection";
export type { UseSSEConnectionOptions, UseSSEConnectionReturn } from "./useSSEConnection";

export { useSSEEventBuffer } from "./useSSEEventBuffer";
export type { UseSSEEventBufferOptions, UseSSEEventBufferReturn } from "./useSSEEventBuffer";

export { useSSEDeduplication } from "./useSSEDeduplication";
export type { UseSSEDeduplicationReturn } from "./useSSEDeduplication";

export { useStreamingAnswerBuffer } from "./useStreamingAnswerBuffer";
export type { UseStreamingAnswerBufferReturn } from "./useStreamingAnswerBuffer";

// Shared types
export type {
  EventState,
  ConnectionState,
  AssistantBufferEntry,
  FlushHandle,
  FlushMode,
} from "./types";

export {
  SLOW_RETRY_INTERVAL_MS,
  FAST_RECONNECT_ATTEMPTS,
  STREAM_FLUSH_MS,
  MAX_EVENT_HISTORY,
  IS_TEST_ENV,
} from "./types";
