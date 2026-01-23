/**
 * Hook for managing SSE connection lifecycle.
 * Handles connection establishment, error handling, and reconnection logic.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import type { MutableRefObject } from "react";
import { SSEClient } from "@/lib/events/sseClient";
import { EventPipeline } from "@/lib/events/eventPipeline";
import { authClient } from "@/lib/auth/client";
import type { SSEReplayMode } from "@/lib/api";
import type { ConnectionState } from "./types";

export interface UseSSEConnectionOptions {
  sessionId: string | null;
  enabled: boolean;
  maxReconnectAttempts: number;
  pipelineRef: MutableRefObject<EventPipeline | null>;
  hasLocalHistoryRef: MutableRefObject<boolean>;
  onConnectionStateChange: (state: ConnectionState) => void;
}

export interface UseSSEConnectionReturn {
  /** Manually trigger a reconnection */
  reconnect: () => void;
  /** Clean up connection resources */
  cleanup: () => void;
  /** Trigger a connection attempt if possible */
  connect: () => void;
}

/**
 * Parse error message from SSE error event.
 */
function parseServerError(err: Event | Error): string | null {
  if (err instanceof MessageEvent) {
    const { data } = err;

    if (typeof data === "string") {
      const trimmed = data.trim();
      if (!trimmed) {
        return null;
      }

      try {
        const parsed = JSON.parse(trimmed);
        if (parsed && typeof parsed.error === "string") {
          return parsed.error;
        }
      } catch {
        // Not JSON, fall back to raw string
      }

      return trimmed;
    }

    if (data && typeof data === "object" && "error" in data) {
      const errorMessage = (data as { error?: unknown }).error;
      if (typeof errorMessage === "string" && errorMessage.trim()) {
        return errorMessage;
      }
    }

    return null;
  }

  if (err instanceof Error) {
    return err.message;
  }

  return null;
}

export function useSSEConnection(
  options: UseSSEConnectionOptions
): UseSSEConnectionReturn {
  const {
    sessionId,
    enabled,
    maxReconnectAttempts,
    pipelineRef,
    hasLocalHistoryRef,
    onConnectionStateChange,
  } = options;

  const clientRef = useRef<SSEClient | null>(null);
  const isConnectingRef = useRef(false);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const sessionIdRef = useRef(sessionId);
  const connectInternalRef = useRef<(() => Promise<void>) | null>(null);
  const isDisposedRef = useRef(false);

  // Keep refs in sync
  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  const cleanup = useCallback(() => {
    isDisposedRef.current = true;
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (clientRef.current) {
      clientRef.current.dispose();
      clientRef.current = null;
    }
    isConnectingRef.current = false;
    reconnectAttemptsRef.current = 0;
  }, []);

  const connectInternal = useCallback(async () => {
    const currentSessionId = sessionIdRef.current;
    const currentEnabled = enabled;
    const pipeline = pipelineRef.current;
    const hasLocalHistory = hasLocalHistoryRef.current;

    if (!currentSessionId || !currentEnabled || !pipeline) {
      return;
    }

    if (isConnectingRef.current || clientRef.current) {
      return;
    }

    // Check if we've exceeded max attempts
    if (reconnectAttemptsRef.current >= maxReconnectAttempts) {
      console.warn(
        "[SSE] Max reconnection attempts reached, stopping auto-reconnect"
      );
      onConnectionStateChange({
        sessionId: currentSessionId,
        isConnected: false,
        isReconnecting: false,
        error: "Maximum reconnection attempts exceeded",
        reconnectAttempts: maxReconnectAttempts,
      });
      return;
    }

    isConnectingRef.current = true;
    isDisposedRef.current = false;

    const token = authClient.getSession()?.accessToken;
    const replay: SSEReplayMode = hasLocalHistory ? "none" : "session";

    const client = new SSEClient(currentSessionId, pipeline, {
      replay,
      onOpen: () => {
        if (isDisposedRef.current) {
          return;
        }
        isConnectingRef.current = false;
        reconnectAttemptsRef.current = 0;
        onConnectionStateChange({
          sessionId: currentSessionId,
          isConnected: true,
          isReconnecting: false,
          error: null,
          reconnectAttempts: 0,
        });
      },
      onError: (err) => {
        console.error("[SSE] Connection error:", err);

        if (isDisposedRef.current) {
          return;
        }

        const serverErrorMessage = parseServerError(err);

        if (clientRef.current) {
          clientRef.current.dispose();
          clientRef.current = null;
        }
        isConnectingRef.current = false;

        if (serverErrorMessage) {
          console.warn(
            "[SSE] Server returned error payload, continuing to reconnect:",
            serverErrorMessage
          );
        }

        const nextAttempts = reconnectAttemptsRef.current + 1;
        reconnectAttemptsRef.current = nextAttempts;
        const clampedAttempts = Math.min(nextAttempts, maxReconnectAttempts);

        if (nextAttempts > maxReconnectAttempts) {
          console.warn("[SSE] Maximum reconnection attempts exceeded");
          onConnectionStateChange({
            sessionId: currentSessionId,
            isConnected: false,
            isReconnecting: false,
            error: "Maximum reconnection attempts exceeded",
            reconnectAttempts: maxReconnectAttempts,
          });
          return;
        }

        const delay = Math.min(1000 * 2 ** (nextAttempts - 1), 30000);
        console.log(
          `[SSE] Scheduling reconnect attempt ${nextAttempts}/${maxReconnectAttempts} in ${delay}ms`
        );
        onConnectionStateChange({
          sessionId: currentSessionId,
          isConnected: false,
          isReconnecting: true,
          error: serverErrorMessage,
          reconnectAttempts: clampedAttempts,
        });

        reconnectTimeoutRef.current = setTimeout(() => {
          if (!isDisposedRef.current) {
            void connectInternalRef.current?.();
          }
        }, delay);
      },
      onClose: () => {
        if (!isDisposedRef.current) {
          onConnectionStateChange({
            sessionId: currentSessionId,
            isConnected: false,
            isReconnecting: false,
            error: null,
            reconnectAttempts: reconnectAttemptsRef.current,
          });
        }
      },
    });

    clientRef.current = client;

    try {
      client.connect(token);
    } catch (err) {
      console.error("[SSE] Failed to connect:", err);
      if (clientRef.current) {
        clientRef.current.dispose();
        clientRef.current = null;
      }
      isConnectingRef.current = false;
      onConnectionStateChange({
        sessionId: currentSessionId,
        isConnected: false,
        isReconnecting: false,
        error: err instanceof Error ? err.message : "Unknown connection error",
        reconnectAttempts: reconnectAttemptsRef.current,
      });
    }
  }, [enabled, maxReconnectAttempts, pipelineRef, hasLocalHistoryRef, onConnectionStateChange]);

  const connect = useCallback(() => {
    void connectInternal();
  }, [connectInternal]);

  // Store connectInternal in ref for use in timeout callbacks
  useEffect(() => {
    connectInternalRef.current = connectInternal;
  }, [connectInternal]);

  const reconnect = useCallback(() => {
    cleanup();
    reconnectAttemptsRef.current = 0;
    isDisposedRef.current = false;
    onConnectionStateChange({
      sessionId: sessionIdRef.current,
      isConnected: false,
      isReconnecting: true,
      error: null,
      reconnectAttempts: 0,
    });
    void connectInternal();
  }, [cleanup, connectInternal, onConnectionStateChange]);

  return {
    connect,
    reconnect,
    cleanup,
  };
}
