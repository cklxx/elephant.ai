/**
 * SSE Connection Hook - Re-exports from modular implementation.
 *
 * This file maintains backwards compatibility with existing imports.
 * The actual implementation has been refactored into the useSSE/ directory
 * with specialized hooks for better separation of concerns:
 *
 * - useSSE/useSSE.ts: Main orchestration hook
 * - useSSE/useSSEConnection.ts: Connection lifecycle management
 * - useSSE/useSSEEventBuffer.ts: Event buffering and flush scheduling
 * - useSSE/useSSEDeduplication.ts: Event deduplication
 * - useSSE/useStreamingAnswerBuffer.ts: Streaming answer accumulation
 *
 * @see useSSE/index.ts for all available exports
 */

export { useSSE } from "./useSSE/useSSE";
export type { UseSSEOptions, UseSSEReturn } from "./useSSE/types";
