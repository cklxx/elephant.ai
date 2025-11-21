/**
 * SSE Connection Hook built on top of a dedicated SSE client and
 * event pipeline.
 *
 * Responsibilities are split as follows:
 * - `SSEClient` manages the browser EventSource connection.
 * - `EventPipeline` validates raw payloads, triggers side effects via
 *   the registry and emits events on the shared bus.
 * - This hook only manages connection state for React consumers and
 *   subscribes to the bus for event updates.
 */

import { useEffect, useRef, useState, useCallback } from "react";
import { AnyAgentEvent, UserTaskEvent } from "@/lib/types";
import { agentEventBus } from "@/lib/events/eventBus";
import { defaultEventRegistry } from "@/lib/events/eventRegistry";
import { resetAttachmentRegistry } from "@/lib/events/attachmentRegistry";
import { EventPipeline } from "@/lib/events/eventPipeline";
import { SSEClient } from "@/lib/events/sseClient";
import { authClient } from "@/lib/auth/client";

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

export function useSSE(
  sessionId: string | null,
  options: UseSSEOptions = {},
): UseSSEReturn {
  const { enabled = true, onEvent, maxReconnectAttempts = 5 } = options;

  const [events, setEvents] = useState<AnyAgentEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [isReconnecting, setIsReconnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const isConnectingRef = useRef(false);
  const clientRef = useRef<SSEClient | null>(null);
  const pipelineRef = useRef<EventPipeline | null>(null);
  const sessionIdRef = useRef(sessionId);
  const dedupeRef = useRef<{ seen: Set<string>; order: string[] }>({
    seen: new Set(),
    order: [],
  });
  const previousSessionIdRef = useRef<string | null>(sessionId);

  const resetDedupe = useCallback(() => {
    dedupeRef.current = {
      seen: new Set(),
      order: [],
    };
  }, []);

  const onEventRef = useRef(onEvent);
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const parseServerError = useCallback((err: Event | Error) => {
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
  }, []);

  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  const clearEvents = useCallback(() => {
    setEvents([]);
    resetAttachmentRegistry();
    resetDedupe();
  }, [resetDedupe]);

  const cleanup = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (clientRef.current) {
      clientRef.current.dispose();
      clientRef.current = null;
    }
    isConnectingRef.current = false;
  }, []);

  const connectInternal = useCallback(async () => {
    const currentSessionId = sessionIdRef.current;
    const currentEnabled = enabled;
    const pipeline = pipelineRef.current;

    if (!currentSessionId || !currentEnabled || !pipeline) {
      return;
    }

    if (isConnectingRef.current || clientRef.current) {
      return;
    }

    // Check if we've exceeded max attempts - stop reconnecting
    if (reconnectAttemptsRef.current >= maxReconnectAttempts) {
      console.warn(
        "[SSE] Max reconnection attempts reached, stopping auto-reconnect",
      );
      setIsReconnecting(false);
      setError("Maximum reconnection attempts exceeded");
      return;
    }

    isConnectingRef.current = true;

    const token = await authClient.ensureAccessToken();
    if (!token) {
      console.warn("[SSE] Missing access token, skipping connection attempt");
      setIsConnected(false);
      setIsReconnecting(false);
      setError("Missing access token");
      isConnectingRef.current = false;
      return;
    }

    const client = new SSEClient(currentSessionId, pipeline, {
      onOpen: () => {
        isConnectingRef.current = false;
        setIsConnected(true);
        setIsReconnecting(false);
        setError(null);
        reconnectAttemptsRef.current = 0;
        setReconnectAttempts(0);
      },
      onError: (err) => {
        console.error("[SSE] Connection error:", err);

        const serverErrorMessage = parseServerError(err);

        if (clientRef.current) {
          clientRef.current.dispose();
          clientRef.current = null;
        }
        isConnectingRef.current = false;
        setIsConnected(false);

        if (serverErrorMessage) {
          console.warn(
            "[SSE] Server returned error payload, continuing to reconnect:",
            serverErrorMessage,
          );
          setError(serverErrorMessage);
        }

        const nextAttempts = reconnectAttemptsRef.current + 1;
        reconnectAttemptsRef.current = nextAttempts;
        const clampedAttempts = Math.min(
          nextAttempts,
          maxReconnectAttempts,
        );
        setReconnectAttempts(clampedAttempts);

        if (nextAttempts > maxReconnectAttempts) {
          console.warn("[SSE] Maximum reconnection attempts exceeded");
          setError("Maximum reconnection attempts exceeded");
          setIsReconnecting(false);
          return;
        }

        const delay = Math.min(1000 * 2 ** (nextAttempts - 1), 30000);
        console.log(
          `[SSE] Scheduling reconnect attempt ${nextAttempts}/${maxReconnectAttempts} in ${delay}ms`,
        );
        setIsReconnecting(true);

        reconnectTimeoutRef.current = setTimeout(() => {
          void connectInternal();
        }, delay);
      },
      onClose: () => {
        setIsConnected(false);
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
      setIsConnected(false);
      setIsReconnecting(false);
      setError(
        err instanceof Error ? err.message : "Unknown connection error",
      );
    }
  }, [enabled, maxReconnectAttempts]);

  useEffect(() => {
    pipelineRef.current = new EventPipeline({
      bus: agentEventBus,
      registry: defaultEventRegistry,
      onInvalidEvent: (raw, validationError) => {
        // Log as warning instead of error for better UX
        // Unknown/invalid events should not break the application
        console.warn("[SSE] Event validation failed (skipping):", {
          raw,
          error: validationError,
          note: "This event will be skipped. This is usually harmless.",
        });
      },
    });

    const unsubscribe = agentEventBus.subscribe((event) => {
      const isStreamingTaskComplete =
        event.event_type === "task_complete" &&
        (Boolean(event.is_streaming) || Boolean(event.stream_finished));

      if (!isStreamingTaskComplete) {
        const dedupeKey = buildEventSignature(event);
        const cache = dedupeRef.current;
        if (cache.seen.has(dedupeKey)) {
          return;
        }

        cache.seen.add(dedupeKey);
        cache.order.push(dedupeKey);
        if (cache.order.length > 2000) {
          const oldest = cache.order.shift();
          if (oldest) {
            cache.seen.delete(oldest);
          }
        }
      }

      setEvents((prev) => {
        if (
          event.event_type === "task_complete" &&
          (event.is_streaming || event.stream_finished)
        ) {
          const matchIndex = findLastStreamingTaskCompleteIndex(prev, event);
          if (matchIndex !== -1) {
            const nextEvents = [...prev];
            nextEvents[matchIndex] = event;
            return nextEvents;
          }
        }

        return [...prev, event];
      });
      onEventRef.current?.(event);
    });

    return () => {
      unsubscribe();
      pipelineRef.current = null;
    };
  }, []);

  useEffect(() => {
    const previousSessionId = previousSessionIdRef.current;
    previousSessionIdRef.current = sessionId;

    cleanup();
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    setError(null);

    const shouldResetState =
      sessionId === null ||
      (Boolean(previousSessionId) && previousSessionId !== sessionId);

    if (shouldResetState) {
      setEvents([]);
      resetAttachmentRegistry();
      resetDedupe();
    }

    if (sessionId && enabled) {
      void connectInternal();
    }

    return () => {
      cleanup();
    };
  }, [sessionId, enabled, connectInternal, cleanup, resetDedupe]);

  const reconnect = useCallback(() => {
    cleanup();
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    void connectInternal();
  }, [cleanup, connectInternal]);

  const addEvent = useCallback((event: AnyAgentEvent) => {
    defaultEventRegistry.run(event);
    agentEventBus.emit(event);
  }, []);

  return {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    clearEvents,
    reconnect,
    addEvent,
  };
}

function findLastStreamingTaskCompleteIndex(
  events: AnyAgentEvent[],
  incoming: AnyAgentEvent,
): number {
  const incomingTaskId =
    incoming.event_type === "task_complete" && "task_id" in incoming
      ? incoming.task_id
      : undefined;

  if (!incomingTaskId) return -1;

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const candidate = events[i];
    if (candidate.event_type !== "task_complete") continue;
    const candidateTaskId = "task_id" in candidate ? candidate.task_id : undefined;
    if (
      candidateTaskId &&
      candidateTaskId === incomingTaskId &&
      candidate.session_id === incoming.session_id
    ) {
      return i;
    }
  }

  return -1;
}

export function buildEventSignature(event: AnyAgentEvent): string {
  // user_task is emitted immediately when a task is created. We may also
  // optimistically add it on the client to ensure it renders even if the SSE
  // stream reconnects. Deduping purely by task/session keeps the UI from
  // showing duplicates when the server replay arrives with a different
  // timestamp.
  if (event.event_type === "user_task") {
    const taskEvent = event as UserTaskEvent;
    return [
      taskEvent.event_type,
      taskEvent.session_id ?? "",
      taskEvent.task_id ?? "",
      taskEvent.task,
    ].join("|");
  }

  const baseParts = [
    event.event_type,
    event.timestamp ?? '',
    event.session_id ?? '',
    'task_id' in event && event.task_id ? event.task_id : '',
  ];

  if ('call_id' in event && event.call_id) {
    baseParts.push(event.call_id);
  }
  if ('iteration' in event && typeof event.iteration === 'number') {
    baseParts.push(String(event.iteration));
  }
  if ('chunk' in event && typeof event.chunk === 'string') {
    baseParts.push(event.chunk);
  }
  if ('delta' in event && typeof event.delta === 'string') {
    baseParts.push(event.delta);
  }
  if ('result' in event && typeof event.result === 'string') {
    baseParts.push(event.result);
  }
  if ('error' in event && typeof event.error === 'string') {
    baseParts.push(event.error);
  }
  if ('final_answer' in event && typeof event.final_answer === 'string') {
    baseParts.push(event.final_answer);
  }
  if ('task' in event && typeof event.task === 'string') {
    baseParts.push(event.task);
  }
  if ('created_at' in event) {
    const createdAt = (event as { created_at?: unknown }).created_at;
    if (typeof createdAt === 'string') {
      baseParts.push(createdAt);
    }
  }

  return baseParts.join('|');
}
