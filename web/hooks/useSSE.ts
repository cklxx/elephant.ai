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
import { type SSEReplayMode } from "@/lib/api";

const STREAM_FLUSH_MS = 16;
const IS_TEST_ENV =
  process.env.NODE_ENV === "test" ||
  process.env.VITEST_WORKER !== undefined;

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

  type EventState = { sessionId: string | null; events: AnyAgentEvent[] };
  type ConnectionState = {
    sessionId: string | null;
    isConnected: boolean;
    isReconnecting: boolean;
    error: string | null;
    reconnectAttempts: number;
  };

  const [eventState, setEventState] = useState<EventState>(() => ({
    sessionId,
    events: [],
  }));
  const [connectionState, setConnectionState] = useState<ConnectionState>(() => ({
    sessionId,
    isConnected: false,
    isReconnecting: false,
    error: null,
    reconnectAttempts: 0,
  }));

  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const isConnectingRef = useRef(false);
  const clientRef = useRef<SSEClient | null>(null);
  const pipelineRef = useRef<EventPipeline | null>(null);
  const connectInternalRef = useRef<(() => Promise<void>) | null>(null);
  const streamingAnswerBufferRef = useRef<Map<string, string>>(new Map());
  const assistantMessageBufferRef = useRef<Map<string, AssistantBufferEntry>>(
    new Map(),
  );
  const hasLocalHistoryRef = useRef(false);
  const initialUserId = authClient.getSession()?.user.id?.trim() || null;
  const userIdRef = useRef<string | null>(initialUserId);
  const sessionIdRef = useRef(sessionId);
  const dedupeRef = useRef<{ seen: Set<string>; order: string[] }>({
    seen: new Set(),
    order: [],
  });
  const previousSessionIdRef = useRef<string | null>(sessionId);
  const pendingEventsRef = useRef<AnyAgentEvent[]>([]);
  const flushHandleRef = useRef<number | null>(null);
  const flushModeRef = useRef<"raf" | "timeout" | null>(null);

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

  const cancelScheduledFlush = useCallback(() => {
    if (flushHandleRef.current == null) {
      return;
    }

    const handle = flushHandleRef.current;
    const mode = flushModeRef.current;
    flushHandleRef.current = null;
    flushModeRef.current = null;

    if (mode === "raf" && typeof window !== "undefined" && "cancelAnimationFrame" in window) {
      window.cancelAnimationFrame(handle);
      return;
    }

    clearTimeout(handle);
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

  const flushPendingEvents = useCallback(() => {
    const pending = pendingEventsRef.current;
    if (pending.length === 0) {
      return;
    }

    pendingEventsRef.current = [];
    const processedEvents: AnyAgentEvent[] = [];

    pending.forEach((event) => {
      const bufferedEvent = mergeStreamingTaskComplete(
        event,
        streamingAnswerBufferRef.current,
      );

      if (eventMatches(bufferedEvent, "workflow.node.output.delta")) {
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
        (enrichedEvent.is_streaming === true ||
          typeof enrichedEvent.stream_finished === "boolean");

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

      const activeSessionId = sessionIdRef.current;
      const enrichedSessionId =
        "session_id" in enrichedEvent && typeof enrichedEvent.session_id === "string"
          ? enrichedEvent.session_id
          : null;

      if (
        activeSessionId &&
        enrichedSessionId &&
        enrichedSessionId !== activeSessionId
      ) {
        return;
      }

      processedEvents.push(enrichedEvent);
      onEventRef.current?.(enrichedEvent);
    });

    if (processedEvents.length === 0) {
      return;
    }

    setEventState((prev) => {
      let nextSessionId = prev.sessionId;
      let nextEvents = prev.events;
      let mutated = false;

      processedEvents.forEach((enrichedEvent) => {
        const activeSessionId = sessionIdRef.current;
        const enrichedSessionId =
          "session_id" in enrichedEvent && typeof enrichedEvent.session_id === "string"
            ? enrichedEvent.session_id
            : null;
        const targetSessionId =
          activeSessionId ?? enrichedSessionId ?? nextSessionId ?? null;

        if (targetSessionId !== nextSessionId) {
          nextSessionId = targetSessionId;
          nextEvents = [];
          mutated = true;
        } else if (!mutated) {
          nextEvents = [...nextEvents];
          mutated = true;
        }

        const isStreamingTaskComplete =
          isWorkflowResultFinalEvent(enrichedEvent) &&
          (enrichedEvent.is_streaming === true ||
            typeof enrichedEvent.stream_finished === "boolean");

        if (isStreamingTaskComplete) {
          const matchIndex = findLastStreamingTaskCompleteIndex(
            nextEvents,
            enrichedEvent,
          );
          if (matchIndex !== -1) {
            nextEvents[matchIndex] = enrichedEvent;
          } else {
            const filtered = nextEvents.filter(
              (evt) =>
                !eventMatches(evt, "workflow.result.final", "workflow.result.final") ||
                !isSameTask(evt, enrichedEvent),
            );
            nextEvents = [...filtered, enrichedEvent];
          }
          return;
        }

        if (mergeDeltaEvent(nextEvents, enrichedEvent)) {
          return;
        }

        nextEvents.push(enrichedEvent);
      });

      const nextState = {
        sessionId: nextSessionId,
        events: clampEvents(squashFinalEvents(nextEvents), MAX_EVENT_HISTORY),
      };

      hasLocalHistoryRef.current = nextState.events.some(
        (evt) => evt.event_type !== "connected",
      );
      return nextState;
    });
  }, [applyAssistantAnswerFallback]);

  const scheduleFlush = useCallback(() => {
    if (IS_TEST_ENV) {
      flushPendingEvents();
      return;
    }

    if (flushHandleRef.current != null) {
      return;
    }

    if (typeof window !== "undefined" && "requestAnimationFrame" in window) {
      flushModeRef.current = "raf";
      flushHandleRef.current = window.requestAnimationFrame(() => {
        flushHandleRef.current = null;
        flushModeRef.current = null;
        flushPendingEvents();
      });
      return;
    }

    flushModeRef.current = "timeout";
    flushHandleRef.current = setTimeout(() => {
      flushHandleRef.current = null;
      flushModeRef.current = null;
      flushPendingEvents();
    }, STREAM_FLUSH_MS);
  }, [flushPendingEvents]);

  const enqueueEvent = useCallback(
    (event: AnyAgentEvent) => {
      pendingEventsRef.current.push(event);
      scheduleFlush();
    },
    [scheduleFlush],
  );

  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  const clearEvents = useCallback(() => {
    cancelScheduledFlush();
    pendingEventsRef.current = [];
    setEventState({ sessionId: sessionIdRef.current, events: [] });
    if (!userIdRef.current) {
      resetAttachmentRegistry();
    }
    resetDedupe();
    resetPipelineDedupe();
    resetStreamingBuffer();
    resetAssistantMessageBuffer();
    hasLocalHistoryRef.current = false;
  }, [
    cancelScheduledFlush,
    resetDedupe,
    resetPipelineDedupe,
    resetStreamingBuffer,
    resetAssistantMessageBuffer,
  ]);

  const cleanup = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    cancelScheduledFlush();
    pendingEventsRef.current = [];
    if (clientRef.current) {
      clientRef.current.dispose();
      clientRef.current = null;
    }
    isConnectingRef.current = false;
  }, [cancelScheduledFlush]);

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
      setConnectionState({
        sessionId: currentSessionId,
        isConnected: false,
        isReconnecting: false,
        error: "Maximum reconnection attempts exceeded",
        reconnectAttempts: maxReconnectAttempts,
      });
      return;
    }

    isConnectingRef.current = true;

    const token = authClient.getSession()?.accessToken;

    const replay: SSEReplayMode = hasLocalHistoryRef.current ? "none" : "session";

    const client = new SSEClient(currentSessionId, pipeline, {
      replay,
      onOpen: () => {
        isConnectingRef.current = false;
        reconnectAttemptsRef.current = 0;
        setConnectionState({
          sessionId: currentSessionId,
          isConnected: true,
          isReconnecting: false,
          error: null,
          reconnectAttempts: 0,
        });
      },
      onError: (err) => {
        console.error("[SSE] Connection error:", err);

        const serverErrorMessage = parseServerError(err);

        if (clientRef.current) {
          clientRef.current.dispose();
          clientRef.current = null;
        }
        isConnectingRef.current = false;

        if (serverErrorMessage) {
          console.warn(
            "[SSE] Server returned error payload, continuing to reconnect:",
            serverErrorMessage,
          );
        }

        const nextAttempts = reconnectAttemptsRef.current + 1;
        reconnectAttemptsRef.current = nextAttempts;
        const clampedAttempts = Math.min(
          nextAttempts,
          maxReconnectAttempts,
        );

        if (nextAttempts > maxReconnectAttempts) {
          console.warn("[SSE] Maximum reconnection attempts exceeded");
          setConnectionState({
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
          `[SSE] Scheduling reconnect attempt ${nextAttempts}/${maxReconnectAttempts} in ${delay}ms`,
        );
        setConnectionState((prev) => ({
          sessionId: currentSessionId,
          isConnected: false,
          isReconnecting: true,
          error:
            serverErrorMessage ??
            (prev.sessionId === currentSessionId ? prev.error : null),
          reconnectAttempts: clampedAttempts,
        }));

        reconnectTimeoutRef.current = setTimeout(() => {
          void connectInternalRef.current?.();
        }, delay);
      },
      onClose: () => {
        setConnectionState((prev) => ({
          sessionId: currentSessionId,
          isConnected: false,
          isReconnecting: prev.sessionId === currentSessionId ? prev.isReconnecting : false,
          error: prev.sessionId === currentSessionId ? prev.error : null,
          reconnectAttempts:
            prev.sessionId === currentSessionId ? prev.reconnectAttempts : 0,
        }));
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
      setConnectionState((prev) => ({
        sessionId: currentSessionId,
        isConnected: false,
        isReconnecting: false,
        error: err instanceof Error ? err.message : "Unknown connection error",
        reconnectAttempts:
          prev.sessionId === currentSessionId ? prev.reconnectAttempts : 0,
      }));
    }
  }, [enabled, maxReconnectAttempts, parseServerError]);

  useEffect(() => {
    connectInternalRef.current = connectInternal;
  }, [connectInternal]);

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
      enqueueEvent(event);
    });

    return () => {
      unsubscribe();
      pipelineRef.current = null;
    };
  }, [applyAssistantAnswerFallback]);

  useEffect(() => {
    const previousSessionId = previousSessionIdRef.current;
    previousSessionIdRef.current = sessionId;

    const currentUserId = authClient.getSession()?.user.id?.trim() || null;
    const userChanged = userIdRef.current !== currentUserId;
    const sessionChanged = Boolean(previousSessionId) && previousSessionId !== sessionId;
    const shouldResetAttachments = sessionChanged || userChanged || !currentUserId;
    userIdRef.current = currentUserId;

    cleanup();
    reconnectAttemptsRef.current = 0;

    const shouldResetState = sessionId === null || sessionChanged;

    if (shouldResetState) {
      if (shouldResetAttachments) {
        resetAttachmentRegistry();
      }
      resetDedupe();
      resetPipelineDedupe();
      resetStreamingBuffer();
      resetAssistantMessageBuffer();
      hasLocalHistoryRef.current = false;
    }

    if (!shouldResetState && shouldResetAttachments) {
      resetAttachmentRegistry();
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
    setConnectionState({
      sessionId: sessionIdRef.current,
      isConnected: false,
      isReconnecting: true,
      error: null,
      reconnectAttempts: 0,
    });
    void connectInternal();
  }, [cleanup, connectInternal]);

  const addEvent = useCallback((event: AnyAgentEvent) => {
    defaultEventRegistry.run(event);
    agentEventBus.emit(event);
  }, []);

  const events =
    Boolean(sessionId) && eventState.sessionId === sessionId
      ? eventState.events
      : [];
  const isConnected =
    Boolean(sessionId) && connectionState.sessionId === sessionId
      ? connectionState.isConnected
      : false;
  const isReconnecting =
    Boolean(sessionId) && connectionState.sessionId === sessionId
      ? connectionState.isReconnecting
      : false;
  const error =
    Boolean(sessionId) && connectionState.sessionId === sessionId
      ? connectionState.error
      : null;
  const reconnectAttempts =
    Boolean(sessionId) && connectionState.sessionId === sessionId
      ? connectionState.reconnectAttempts
      : 0;

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

const MAX_STREAM_DELTA_CHARS = 10_000;

function mergeDeltaEvent(
  events: AnyAgentEvent[],
  incoming: AnyAgentEvent,
): boolean {
  if (!eventMatches(incoming, "workflow.node.output.delta")) {
    return false;
  }

  const last = events[events.length - 1];
  if (!last || !eventMatches(last, "workflow.node.output.delta")) {
    return false;
  }

  const lastNodeId = typeof (last as any).node_id === "string" ? (last as any).node_id : "";
  const incomingNodeId =
    typeof (incoming as any).node_id === "string" ? (incoming as any).node_id : "";

  if ((lastNodeId || incomingNodeId) && lastNodeId !== incomingNodeId) {
    return false;
  }

  const lastSessionId = typeof last.session_id === "string" ? last.session_id : "";
  const incomingSessionId =
    typeof incoming.session_id === "string" ? incoming.session_id : "";
  if (lastSessionId !== incomingSessionId) {
    return false;
  }

  if ((last.task_id ?? "") !== (incoming.task_id ?? "")) {
    return false;
  }

  if ((last.parent_task_id ?? "") !== (incoming.parent_task_id ?? "")) {
    return false;
  }

  if ((last.agent_level ?? "") !== (incoming.agent_level ?? "")) {
    return false;
  }

  const lastDelta = typeof (last as any).delta === "string" ? (last as any).delta : "";
  const incomingDelta =
    typeof (incoming as any).delta === "string" ? (incoming as any).delta : "";
  const mergedDeltaRaw = incomingDelta ? `${lastDelta}${incomingDelta}` : lastDelta;
  const mergedDelta =
    mergedDeltaRaw.length > MAX_STREAM_DELTA_CHARS
      ? mergedDeltaRaw.slice(-MAX_STREAM_DELTA_CHARS)
      : mergedDeltaRaw;

  const merged = {
    ...(last as any),
    ...(incoming as any),
    delta: mergedDelta,
    timestamp: incoming.timestamp ?? last.timestamp,
  } as AnyAgentEvent;

  events[events.length - 1] = merged;
  return true;
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
