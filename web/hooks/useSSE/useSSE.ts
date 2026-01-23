/**
 * SSE Connection Hook built on top of a dedicated SSE client and
 * event pipeline.
 *
 * Responsibilities are split as follows:
 * - `SSEClient` manages the browser EventSource connection.
 * - `EventPipeline` validates raw payloads, triggers side effects via
 *   the registry and emits events on the shared bus.
 * - This hook orchestrates connection state, event processing, and
 *   state management for React consumers.
 *
 * This refactored version composes specialized hooks:
 * - useSSEDeduplication: Event deduplication
 * - useSSEEventBuffer: Event buffering and flush scheduling
 * - useStreamingAnswerBuffer: Streaming answer accumulation
 * - useSSEConnection: Connection lifecycle management
 */

import { useEffect, useRef, useState, useCallback } from "react";
import type { AnyAgentEvent, WorkflowNodeOutputDeltaEvent } from "@/lib/types";
import { isEventType, isStreamingEvent } from "@/lib/events/matching";
import { isWorkflowResultFinalEvent } from "@/lib/typeGuards";
import { agentEventBus } from "@/lib/events/eventBus";
import { defaultEventRegistry } from "@/lib/events/eventRegistry";
import { handleAttachmentEvent, resetAttachmentRegistry } from "@/lib/events/attachmentRegistry";
import { EventPipeline } from "@/lib/events/eventPipeline";
import { authClient } from "@/lib/auth/client";

import { useSSEDeduplication } from "./useSSEDeduplication";
import { useSSEEventBuffer } from "./useSSEEventBuffer";
import { useStreamingAnswerBuffer } from "./useStreamingAnswerBuffer";
import { useSSEConnection } from "./useSSEConnection";
import type { UseSSEOptions, UseSSEReturn, EventState, ConnectionState, MAX_EVENT_HISTORY } from "./types";

const MAX_STREAM_DELTA_CHARS = 10_000;
const DEFAULT_MAX_EVENT_HISTORY = 1000;

export function useSSE(
  sessionId: string | null,
  options: UseSSEOptions = {}
): UseSSEReturn {
  const { enabled = true, onEvent, maxReconnectAttempts = 5 } = options;

  // State
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

  // Refs
  const pipelineRef = useRef<EventPipeline | null>(null);
  const hasLocalHistoryRef = useRef(false);
  const sessionIdRef = useRef(sessionId);
  const previousSessionIdRef = useRef<string | null>(sessionId);
  const initialUserId = authClient.getSession()?.user.id?.trim() || null;
  const userIdRef = useRef<string | null>(initialUserId);
  const onEventRef = useRef(onEvent);

  // Keep refs in sync
  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  // Compose specialized hooks
  const { dedupeEvent, resetDedupe } = useSSEDeduplication();
  const {
    mergeStreamingTaskComplete,
    trackAssistantMessage,
    applyAssistantAnswerFallback,
    resetStreamingBuffer,
    resetAssistantMessageBuffer,
  } = useStreamingAnswerBuffer();

  const resetPipelineDedupe = useCallback(() => {
    pipelineRef.current?.reset();
  }, []);

  // Process a batch of events after flush
  const processEventBatch = useCallback(
    (pendingEvents: AnyAgentEvent[]) => {
      const processedEvents: AnyAgentEvent[] = [];

      pendingEvents.forEach((event) => {
        // Apply streaming buffer merge
        const bufferedEvent = mergeStreamingTaskComplete(event);

        // Track assistant messages for delta events
        if (isStreamingEvent(bufferedEvent)) {
          trackAssistantMessage(bufferedEvent as WorkflowNodeOutputDeltaEvent);
        }

        // Apply assistant answer fallback
        const enrichedEvent = applyAssistantAnswerFallback(bufferedEvent);

        // Handle attachment events
        if (isEventType(enrichedEvent, "workflow.result.final")) {
          handleAttachmentEvent(enrichedEvent);
        }

        // Check if this is a streaming task complete event (bypass dedup)
        const isStreamingTaskComplete =
          isWorkflowResultFinalEvent(enrichedEvent) &&
          (enrichedEvent.is_streaming === true ||
            typeof enrichedEvent.stream_finished === "boolean");

        // Deduplicate non-streaming events
        if (!isStreamingTaskComplete) {
          if (dedupeEvent(enrichedEvent)) {
            return; // Skip duplicate
          }
        }

        // Filter by session ID
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
          return; // Wrong session
        }

        processedEvents.push(enrichedEvent);
        onEventRef.current?.(enrichedEvent);
      });

      if (processedEvents.length === 0) {
        return;
      }

      // Update event state
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
              enrichedEvent
            );
            if (matchIndex !== -1) {
              nextEvents[matchIndex] = enrichedEvent;
            } else {
              const filtered = nextEvents.filter(
                (evt) =>
                  !isEventType(evt, "workflow.result.final") ||
                  !isSameTask(evt, enrichedEvent)
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
          events: clampEvents(squashFinalEvents(nextEvents), DEFAULT_MAX_EVENT_HISTORY),
        };

        hasLocalHistoryRef.current = nextState.events.some(
          (evt) => evt.event_type !== "connected"
        );
        return nextState;
      });
    },
    [
      mergeStreamingTaskComplete,
      trackAssistantMessage,
      applyAssistantAnswerFallback,
      dedupeEvent,
    ]
  );

  // Event buffer with flush callback
  const { enqueueEvent, cancelScheduledFlush, clearBuffer } = useSSEEventBuffer({
    onFlush: processEventBatch,
  });

  // Connection state change handler
  const handleConnectionStateChange = useCallback((state: ConnectionState) => {
    setConnectionState((prev) => ({
      ...state,
      error: state.error ?? (prev.sessionId === state.sessionId ? prev.error : null),
    }));
  }, []);

  // Connection management
  const { connect, reconnect, cleanup } = useSSEConnection({
    sessionId,
    enabled,
    maxReconnectAttempts,
    pipelineRef,
    hasLocalHistoryRef,
    onConnectionStateChange: handleConnectionStateChange,
  });

  // Initialize pipeline and event bus subscription
  useEffect(() => {
    pipelineRef.current = new EventPipeline({
      bus: agentEventBus,
      registry: defaultEventRegistry,
      onInvalidEvent: (raw, validationError) => {
        console.warn("[SSE] Event validation failed (skipping):", {
          raw,
          error: validationError,
          note: "This event will be skipped. This is usually harmless.",
        });
      },
    });
    connect();

    const unsubscribe = agentEventBus.subscribe((event) => {
      enqueueEvent(event);
    });

    return () => {
      unsubscribe();
      pipelineRef.current = null;
    };
  }, [enqueueEvent, connect]);

  // Handle session changes
  useEffect(() => {
    const previousSessionId = previousSessionIdRef.current;
    previousSessionIdRef.current = sessionId;

    const currentUserId = authClient.getSession()?.user.id?.trim() || null;
    const userChanged = userIdRef.current !== currentUserId;
    const sessionChanged = Boolean(previousSessionId) && previousSessionId !== sessionId;
    const shouldResetAttachments = sessionChanged || userChanged || !currentUserId;
    userIdRef.current = currentUserId;

    cleanup();

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
      connect();
    }
  }, [
    sessionId,
    enabled,
    cleanup,
    connect,
    resetDedupe,
    resetPipelineDedupe,
    resetStreamingBuffer,
    resetAssistantMessageBuffer,
  ]);

  // Clear events
  const clearEvents = useCallback(() => {
    cancelScheduledFlush();
    clearBuffer();
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
    clearBuffer,
    resetDedupe,
    resetPipelineDedupe,
    resetStreamingBuffer,
    resetAssistantMessageBuffer,
  ]);

  // Add event manually
  const addEvent = useCallback((event: AnyAgentEvent) => {
    defaultEventRegistry.run(event);
    agentEventBus.emit(event);
  }, []);

  // Derive return values based on session match
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

// Helper functions

function clampEvents(events: AnyAgentEvent[], maxHistory: number): AnyAgentEvent[] {
  if (events.length <= maxHistory) {
    return events;
  }
  return events.slice(events.length - maxHistory);
}

function findLastStreamingTaskCompleteIndex(
  events: AnyAgentEvent[],
  incoming: AnyAgentEvent
): number {
  const incomingTaskId =
    isEventType(incoming, "workflow.result.final") && "task_id" in incoming
      ? incoming.task_id
      : undefined;

  if (!incomingTaskId) return -1;

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const candidate = events[i];
    if (!isEventType(candidate, "workflow.result.final")) continue;
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
  const taskA = "task_id" in a ? a.task_id : undefined;
  const taskB = "task_id" in b ? b.task_id : undefined;
  const sessionA = "session_id" in a ? a.session_id : undefined;
  const sessionB = "session_id" in b ? b.session_id : undefined;
  return Boolean(
    taskA && taskB && sessionA && sessionB && taskA === taskB && sessionA === sessionB
  );
}

function squashFinalEvents(events: AnyAgentEvent[]): AnyAgentEvent[] {
  const seenTasks = new Set<string>();
  const result: AnyAgentEvent[] = [];

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const evt = events[i];
    if (
      isEventType(evt, "workflow.result.final") &&
      "task_id" in evt &&
      "session_id" in evt
    ) {
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

function mergeDeltaEvent(
  events: AnyAgentEvent[],
  incoming: AnyAgentEvent
): boolean {
  if (!isEventType(incoming, "workflow.node.output.delta")) {
    return false;
  }

  const last = events[events.length - 1];
  if (!last || !isEventType(last, "workflow.node.output.delta")) {
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

// Re-export types for convenience
export type { UseSSEOptions, UseSSEReturn } from "./types";
