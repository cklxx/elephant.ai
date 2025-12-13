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
import { AnyAgentEvent, WorkflowNodeOutputDeltaEvent, eventMatches } from "@/lib/types";
import { isWorkflowResultFinalEvent } from "@/lib/typeGuards";
import { agentEventBus } from "@/lib/events/eventBus";
import { defaultEventRegistry } from "@/lib/events/eventRegistry";
import { handleAttachmentEvent, resetAttachmentRegistry } from "@/lib/events/attachmentRegistry";
import { EventPipeline } from "@/lib/events/eventPipeline";
import { SSEClient } from "@/lib/events/sseClient";
import { buildEventSignature } from "@/lib/events/signature";
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
  const MAX_EVENT_HISTORY = 1000;
  const { enabled = true, onEvent, maxReconnectAttempts = 5 } = options;

  const [events, setEvents] = useState<AnyAgentEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [isReconnecting, setIsReconnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const isConnectingRef = useRef(false);
  const clientRef = useRef<SSEClient | null>(null);
  const pipelineRef = useRef<EventPipeline | null>(null);
  const streamingAnswerBufferRef = useRef<Map<string, string>>(new Map());
  const assistantMessageBufferRef = useRef<Map<string, AssistantBufferEntry>>(
    new Map(),
  );
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

  const resetPipelineDedupe = useCallback(() => {
    pipelineRef.current?.reset();
  }, []);

  const resetStreamingBuffer = useCallback(() => {
    streamingAnswerBufferRef.current.clear();
  }, []);

  const resetAssistantMessageBuffer = useCallback(() => {
    assistantMessageBufferRef.current.clear();
  }, []);

  const onEventRef = useRef(onEvent);
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const applyAssistantAnswerFallback = useCallback(
    (event: AnyAgentEvent): AnyAgentEvent => {
      if (!isWorkflowResultFinalEvent(event)) return event;

      const taskId =
        "task_id" in event && typeof event.task_id === "string"
          ? event.task_id
          : undefined;
      const sessionId =
        "session_id" in event && typeof event.session_id === "string"
          ? event.session_id
          : "";

      if (!taskId) return event;

      const key = `${sessionId}|${taskId}`;
      const entry = assistantMessageBufferRef.current.get(key);
      const existingAnswer =
        typeof event.final_answer === "string" ? event.final_answer : "";
      const streamFinished =
        event.stream_finished === true || event.is_streaming === false;

      if (streamFinished) {
        assistantMessageBufferRef.current.delete(key);
      }

      if (existingAnswer.trim().length > 0) {
        return event;
      }

      if (entry && entry.content.trim().length > 0) {
        if (streamFinished) {
          assistantMessageBufferRef.current.delete(key);
        }
        return {
          ...event,
          final_answer: entry.content,
        };
      }

      return event;
    },
    [],
  );

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
    resetPipelineDedupe();
    resetStreamingBuffer();
    resetAssistantMessageBuffer();
  }, [resetDedupe, resetPipelineDedupe, resetStreamingBuffer, resetAssistantMessageBuffer]);

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
      const bufferedEvent = mergeStreamingTaskComplete(
        event,
        streamingAnswerBufferRef.current,
      );
      if (eventMatches(bufferedEvent, "workflow.node.output.delta", "workflow.node.output.delta", "workflow.node.output.delta")) {
        trackAssistantMessage(
          bufferedEvent as WorkflowNodeOutputDeltaEvent,
          assistantMessageBufferRef.current,
        );
      }

      const enrichedEvent = applyAssistantAnswerFallback(bufferedEvent);

      if (eventMatches(enrichedEvent, "workflow.result.final", "workflow.result.final")) {
        handleAttachmentEvent(enrichedEvent);
      }

      const isStreamingTaskComplete =
        isWorkflowResultFinalEvent(enrichedEvent) &&
        (Boolean(enrichedEvent.is_streaming) || Boolean(enrichedEvent.stream_finished));

      if (!isStreamingTaskComplete) {
        const dedupeKey = buildEventSignature(enrichedEvent);
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
        let nextEvents = prev;

        if (
          isWorkflowResultFinalEvent(enrichedEvent) &&
          (enrichedEvent.is_streaming || enrichedEvent.stream_finished)
        ) {
          const matchIndex = findLastStreamingTaskCompleteIndex(
            prev,
            enrichedEvent,
          );
          if (matchIndex !== -1) {
            nextEvents = [...prev];
            nextEvents[matchIndex] = enrichedEvent;
          } else {
            const filtered = prev.filter(
              (evt) =>
                !eventMatches(evt, "workflow.result.final", "workflow.result.final") ||
                !isSameTask(evt, enrichedEvent),
            );
            nextEvents = [...filtered, enrichedEvent];
          }
        } else {
          nextEvents = [...prev, enrichedEvent];
        }

        return clampEvents(squashFinalEvents(nextEvents), MAX_EVENT_HISTORY);
      });
      onEventRef.current?.(enrichedEvent);
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
      resetPipelineDedupe();
      resetStreamingBuffer();
      resetAssistantMessageBuffer();
    }

    if (sessionId && enabled) {
      void connectInternal();
    }

    return () => {
      cleanup();
    };
  }, [
    sessionId,
    enabled,
    connectInternal,
    cleanup,
    resetDedupe,
    resetPipelineDedupe,
    resetStreamingBuffer,
    resetAssistantMessageBuffer,
  ]);

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

function clampEvents(events: AnyAgentEvent[], maxHistory: number): AnyAgentEvent[] {
  if (events.length <= maxHistory) {
    return events;
  }
  return events.slice(events.length - maxHistory);
}

function findLastStreamingTaskCompleteIndex(
  events: AnyAgentEvent[],
  incoming: AnyAgentEvent,
): number {
  const incomingTaskId =
    eventMatches(incoming, "workflow.result.final", "workflow.result.final") && "task_id" in incoming
      ? incoming.task_id
      : undefined;

  if (!incomingTaskId) return -1;

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const candidate = events[i];
    if (!eventMatches(candidate, "workflow.result.final", "workflow.result.final")) continue;
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

function isSameTask(a: AnyAgentEvent, b: AnyAgentEvent): boolean {
  const taskA = 'task_id' in a ? a.task_id : undefined;
  const taskB = 'task_id' in b ? b.task_id : undefined;
  const sessionA = 'session_id' in a ? a.session_id : undefined;
  const sessionB = 'session_id' in b ? b.session_id : undefined;
  return Boolean(taskA && taskB && sessionA && sessionB && taskA === taskB && sessionA === sessionB);
}

function squashFinalEvents(events: AnyAgentEvent[]): AnyAgentEvent[] {
  const seenTasks = new Set<string>();
  const result: AnyAgentEvent[] = [];

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const evt = events[i];
    if (eventMatches(evt, "workflow.result.final", "workflow.result.final") && "task_id" in evt && "session_id" in evt) {
      const key = `${evt.session_id}|${evt.task_id}`;
      if (seenTasks.has(key)) {
        continue;
      }
      seenTasks.add(key);
    }
    result.push(evt);
  }

  return result.reverse();
}

function mergeStreamingTaskComplete(
  event: AnyAgentEvent,
  buffer: Map<string, string>,
): AnyAgentEvent {
  if (!isWorkflowResultFinalEvent(event)) return event;

  const taskId = event.task_id;
  const sessionId = event.session_id ?? "";

  if (!taskId) return event;

  const key = `${sessionId}|${taskId}`;
  const chunk = event.final_answer ?? "";
  const previous = buffer.get(key) ?? "";
  const isStreaming = event.is_streaming === true;
  const streamFinished = event.stream_finished === true;
  const streamInProgress = isStreaming || event.stream_finished === false;

  if (streamInProgress) {
    const combined = combineChunks(previous, chunk);
    buffer.set(key, combined);
    return {
      ...event,
      final_answer: combined,
    };
  }

  if (streamFinished) {
    const combined = combineChunks(previous, chunk);
    buffer.delete(key);
    return {
      ...event,
      final_answer: combined,
    };
  }

  if (previous) {
    const combined = combineChunks(previous, chunk);
    buffer.delete(key);
    return {
      ...event,
      final_answer: combined,
    };
  }

  return event;
}

type AssistantBufferEntry = {
  iteration: number;
  content: string;
};

function trackAssistantMessage(
  event: WorkflowNodeOutputDeltaEvent,
  buffer: Map<string, AssistantBufferEntry>,
) {
  const taskId =
    "task_id" in event && typeof event.task_id === "string"
      ? event.task_id
      : undefined;
  const sessionId =
    "session_id" in event && typeof event.session_id === "string"
      ? event.session_id
      : "";
  if (!taskId) return;

  const iteration =
    typeof event.iteration === "number" ? event.iteration : Number.MIN_SAFE_INTEGER;
  const key = `${sessionId}|${taskId}`;
  const existing = buffer.get(key);

  const baseContent =
    existing && existing.iteration === iteration ? existing.content : "";
  const combined = combineChunks(baseContent, event.delta ?? "");

  buffer.set(key, {
    iteration,
    content: combined,
  });
}

function combineChunks(previous: string, chunk: string): string {
  if (!chunk) return previous;
  if (!previous) return chunk;

  if (chunk.startsWith(previous)) {
    return chunk;
  }

  if (previous.endsWith(chunk)) {
    return previous;
  }

  return previous + chunk;
}
