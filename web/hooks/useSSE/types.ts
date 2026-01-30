/**
 * Shared types for the useSSE hook family.
 */

import type { AnyAgentEvent } from "@/lib/types";

export interface UseSSEOptions {
  enabled?: boolean;
  onEvent?: (event: AnyAgentEvent) => void;
  maxReconnectAttempts?: number;
}

export interface UseSSEReturn {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  clearEvents: () => void;
  reconnect: () => void;
  addEvent: (event: AnyAgentEvent) => void;
}

export interface EventState {
  sessionId: string | null;
  events: AnyAgentEvent[];
}

export interface ConnectionState {
  sessionId: string | null;
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
}

export interface AssistantBufferEntry {
  iteration: number;
  content: string;
}

export type FlushHandle = ReturnType<typeof setTimeout> | number;
export type FlushMode = "raf" | "timeout" | null;

export const STREAM_FLUSH_MS = 16;
export const MAX_EVENT_HISTORY = 1000;
export const IS_TEST_ENV =
  process.env.NODE_ENV === "test" ||
  process.env.VITEST_WORKER !== undefined;
