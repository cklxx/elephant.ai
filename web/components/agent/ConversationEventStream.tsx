"use client";

import { useMemo } from "react";
import type {
  AnyAgentEvent,
  WorkflowNodeOutputDeltaEvent,
  WorkflowToolCompletedEvent,
  WorkflowToolStartedEvent,
} from "@/lib/types";
import { isEventType } from "@/lib/events/matching";
import { ConnectionBanner } from "./ConnectionBanner";
import { LoadingDots } from "@/components/ui/loading-states";
import {
  EventLine,
  SubagentContext,
  getSubagentContext,
} from "./EventLine";
import { isSubagentLike } from "@/lib/subagent";
import { ClarifyTimeline, type ClarifyTaskGroup } from "./ClarifyTimeline";
import { isOrchestratorRetryMessage } from "@/lib/utils";
import { AgentCard } from "./AgentCard";
import { subagentThreadToCardData } from "./AgentCard/utils";
import { ToolOutputCard } from "./ToolOutputCard";
import { cn } from "@/lib/utils";
import Image from "next/image";
import { AlexWordmark } from "@/components/ui/alex-wordmark";

interface ConversationEventStreamProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
  isRunning?: boolean;
  /** Optimistic messages to display before backend confirmation */
  optimisticMessages?: Array<{ id: string; content: string; timestamp: string }>;
}

export function ConversationEventStream({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  isRunning = false,
  optimisticMessages = [],
}: ConversationEventStreamProps) {
  const activeTaskId = useMemo(() => resolveActiveTaskId(events), [events]);
  const { displayEvents, subagentThreadsByParentId, pendingToolCalls, pendingNodes } = useMemo(
    () =>
      partitionEvents(events, {
        includeDeltas: isRunning,
        activeTaskId,
      }),
    [events, isRunning, activeTaskId],
  );

  const displayEntries = useMemo(
    () => buildDisplayEntriesWithClarifyTimeline(displayEvents),
    [displayEvents],
  );

  const resolvePairedToolStart = useMemo(() => {
    return (event: AnyAgentEvent) => {
      if (!isEventType(event, "workflow.tool.completed")) {
        return undefined;
      }
      const callId = event.call_id;
      const sessionId = typeof event.session_id === "string" ? event.session_id : "";
      if (!callId) {
        return undefined;
      }
      return pendingToolCalls.get(`${sessionId}:${callId}`);
    };
  }, [pendingToolCalls]);

  const combinedEntries = useMemo(
    () => buildInterleavedEntries(displayEntries),
    [displayEntries]
  );

  if (!isConnected || error) {
    return (
      <ConnectionBanner
        isConnected={isConnected}
        isReconnecting={isReconnecting}
        error={error}
        reconnectAttempts={reconnectAttempts}
        onReconnect={onReconnect}
      />
    );
  }

  return (
    <div
      className="flex flex-col w-full max-w-4xl mx-auto pb-12"
      data-testid="conversation-stream"
    >

      <div className="flex flex-col" data-testid="conversation-events">
        {/* Render optimistic messages (user messages waiting for backend confirmation) */}
        {optimisticMessages.map((msg) => (
          <div
            key={`optimistic-${msg.id}`}
            className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2 opacity-70"
            data-testid="optimistic-message"
          >
            <div className="py-2 flex justify-end">
              <div className="flex w-full max-w-[min(36rem,100%)] flex-col items-end gap-2">
                <div className="w-fit max-w-full rounded-2xl border border-border/60 bg-background px-4 py-3 shadow-sm">
                  <div className="text-base text-foreground">
                    <p className="whitespace-pre-wrap leading-normal">{msg.content}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ))}

        {combinedEntries.map((entry, index) => {
          if (entry.kind === "clarifyTimeline") {
            return (
              <div
                key={`clarify-timeline-${entry.ts}-${index}`}
                className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
                data-testid="clarify-timeline-entry"
              >
                <ClarifyTimeline
                  groups={entry.groups}
                  isRunning={isRunning}
                  resolvePairedToolStart={resolvePairedToolStart}
                />
              </div>
            );
          }

          const event = entry.event;
          const key = getStableEventKey(event, index);

          // Check if this is a subagent tool call that should show a card
          // Use task_id (which is the parent_task_id of subagent events) to find the thread
          const parentTaskId = isSubagentToolEvent(event) && typeof event.task_id === "string" ? event.task_id : null;
          const subagentThread = parentTaskId ? subagentThreadsByParentId.get(parentTaskId) : null;
          const hasSubagentCard = subagentThread && subagentThread.events.length > 0;

          return (
            <div
              key={key}
              className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
            >

              <EventLine
                event={event}
                pairedToolStartEvent={resolvePairedToolStart(event)}
              />
              {hasSubagentCard && (
                <div className="mt-2 mb-2" data-testid="subagent-card-container">
                  <AgentCard
                    data={subagentThreadToCardData(
                      subagentThread.key,
                      subagentThread.context,
                      subagentThread.events,
                      subagentThread.subtaskIndex,
                    )}
                    resolvePairedToolStart={resolvePairedToolStart}
                    className="mx-0 my-0"
                  />
                </div>
              )}
            </div>
          );
        })}
        {/* Render pending tool calls (running state) */}
        {Array.from(pendingToolCalls.values()).map((pendingTool) => (
          <div
            key={`pending-tool-${pendingTool.call_id}`}
            className={cn("py-1 pl-2 border-primary/10", "group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2")}
            data-testid="pending-tool-call"
          >
            <ToolOutputCard
              toolName={pendingTool.tool_name}
              parameters={pendingTool.arguments}
              timestamp={pendingTool.timestamp}
              callId={pendingTool.call_id}
              status="running"
            />
          </div>
        ))}
        {/* Render pending nodes (running state) */}
        {Array.from(pendingNodes.values()).map((pendingNode) => (
          <div
            key={`pending-node-${(pendingNode as any).node_id || pendingNode.timestamp}`}
            className={cn("py-1 pl-2 border-primary/10", "group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2")}
            data-testid="pending-node"
          >
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <LoadingDots />
              <span>Running: {(pendingNode as any).node_id || 'node'}</span>
            </div>
          </div>
        ))}
        {isRunning && (
          <div
            className="mt-4 flex items-center text-muted-foreground"
            aria-live="polite"
            data-testid="workflow-running-indicator"
          >
            <LoadingDots />
            <span className="sr-only">Workflow running</span>
          </div>
        )}
      </div>
    </div>
  );
}

interface SubagentThread {
  key: string;
  context: SubagentContext;
  events: AnyAgentEvent[];
  subtaskIndex: number;
  firstSeenAt: number | null;
  firstArrival: number;
}

function parseEventTimestamp(event: AnyAgentEvent): number | null {
  const ts = Date.parse(event.timestamp ?? "");
  return Number.isFinite(ts) ? ts : null;
}

function shouldSkipEvent(event: AnyAgentEvent): boolean {
  // Skip prepare node events - these are internal setup events
  if (
    event.event_type === "workflow.node.started" &&
    (event as any).node_id === "prepare"
  ) {
    return true;
  }

  // Skip diagnostic events
  if (event.event_type.startsWith("workflow.diagnostic")) {
    return true;
  }

  // Skip node lifecycle events (started/completed) - only show summaries and results
  if (
    event.event_type === "workflow.node.started" ||
    event.event_type === "workflow.node.completed"
  ) {
    return true;
  }

  // Skip tool started events - they are merged with completed events or shown as pending
  // But keep subagent tool started events (they should be shown in the subagent card)
  if (event.event_type === "workflow.tool.started" && event.agent_level !== "subagent") {
    return true;
  }

  // Subagent internal events are shown in the subagent card, not in main stream
  // But subagent tool calls (anchor events) are shown
  if (event.agent_level === "subagent") {
    return !isSubagentToolEvent(event);
  }

  return false;
}

function partitionEvents(
  events: AnyAgentEvent[],
  options: { includeDeltas: boolean; activeTaskId: string | null },
): {
  displayEvents: AnyAgentEvent[];
  subagentThreadsByParentId: Map<string, SubagentThread>;
  pendingToolCalls: Map<string, WorkflowToolStartedEvent>;
  pendingNodes: Map<string, AnyAgentEvent>;
} {
  const displayEvents: AnyAgentEvent[] = [];
  const threads = new Map<string, SubagentThread>();
  const arrivalOrder = new WeakMap<AnyAgentEvent, number>();
  const startedTools = new Map<string, WorkflowToolStartedEvent>();
  const completedToolIds = new Set<string>();
  const startedNodes = new Map<string, AnyAgentEvent>();
  const completedNodeIds = new Set<string>();
  let arrival = 0;
  const includeDeltas = options.includeDeltas;
  const activeTaskId = options.activeTaskId;

  // First pass: collect all started and completed tool/node calls
  events.forEach((event) => {
    if (isEventType(event, "workflow.tool.started")) {
      const key = `${event.session_id}:${event.call_id}`;
      startedTools.set(key, event as WorkflowToolStartedEvent);
    } else if (isEventType(event, "workflow.tool.completed")) {
      const key = `${event.session_id}:${event.call_id}`;
      completedToolIds.add(key);
    } else if (isEventType(event, "workflow.node.started")) {
      const key = `${event.session_id}:${(event as any).node_id || event.timestamp}`;
      startedNodes.set(key, event);
    } else if (isEventType(event, "workflow.node.completed")) {
      const key = `${event.session_id}:${(event as any).node_id || event.timestamp}`;
      completedNodeIds.add(key);
    }
  });

  // Find pending tool calls (started but not completed)
  const pendingToolCalls = new Map<string, WorkflowToolStartedEvent>();
  startedTools.forEach((started, key) => {
    if (!completedToolIds.has(key)) {
      pendingToolCalls.set(key, started);
    }
  });

  // Find pending node calls (started but not completed)
  const pendingNodes = new Map<string, AnyAgentEvent>();
  startedNodes.forEach((started, key) => {
    if (!completedNodeIds.has(key)) {
      pendingNodes.set(key, started);
    }
  });

  events.forEach((event) => {
    arrival += 1;
    arrivalOrder.set(event, arrival);

    const eventTs = parseEventTimestamp(event);

    // Handle subagent events - aggregate them into threads
    if (isSubagentLike(event)) {
      const key = getSubagentKey(event);
      const context = getSubagentContext(event);
      const subtaskIndex = getSubtaskIndex(event);

      if (!threads.has(key)) {
        threads.set(key, {
          key,
          context,
          events: [],
          subtaskIndex,
          firstSeenAt: eventTs,
          firstArrival: arrival,
        });
      }

      const thread = threads.get(key)!;
      thread.context = mergeSubagentContext(thread.context, context);
      if (eventTs !== null && (thread.firstSeenAt === null || eventTs < thread.firstSeenAt)) {
        thread.firstSeenAt = eventTs;
      }
      if (arrival < thread.firstArrival) {
        thread.firstArrival = arrival;
      }

      // Add displayable subagent events to the thread
      if (shouldDisplaySubagentEvent(event)) {
        thread.events.push(event);
      }
    }

    // Handle delta events - merge if possible
    if (isEventType(event, "workflow.node.output.delta")) {
      if (
        includeDeltas &&
        event.agent_level !== "subagent" &&
        (activeTaskId === null ||
          (typeof event.task_id === "string" && event.task_id === activeTaskId))
      ) {
        if (!maybeMergeDeltaEvent(displayEvents, event)) {
          displayEvents.push(event);
        }
      }
      return;
    }

    // Add non-skipped events to display
    if (!shouldSkipEvent(event)) {
      displayEvents.push(event);
    }
  });

  // Group threads by parent_task_id for lookup by anchor events
  const subagentThreadsByParentId = new Map<string, SubagentThread>();
  Array.from(threads.values()).forEach((thread) => {
    // Extract parent_task_id from thread key (format: parentTaskId:taskId or parentTaskId:call:callId)
    const parentTaskId = thread.key.split(':')[0];
    if (!parentTaskId) return;

    // Merge threads with same parent_task_id (take the one with most events, or earliest)
    const existing = subagentThreadsByParentId.get(parentTaskId);
    if (!existing || thread.events.length > existing.events.length) {
      subagentThreadsByParentId.set(parentTaskId, {
        ...thread,
        events: sortSubagentEvents(thread.events, arrivalOrder),
      });
    }
  });

  return {
    displayEvents,
    subagentThreadsByParentId,
    pendingToolCalls,
    pendingNodes,
  };
}

function maybeMergeDeltaEvent(
  events: AnyAgentEvent[],
  incoming: AnyAgentEvent,
): boolean {
  if (!isEventType(incoming, "workflow.node.output.delta")) {
    return false;
  }

  const delta = incoming.delta;
  if (typeof delta !== "string" || delta.length === 0) {
    const last = events[events.length - 1];
    if (last && isEventType(last, "workflow.node.output.delta")) {
      const merged: WorkflowNodeOutputDeltaEvent = {
        ...last,
        ...incoming,
        delta: last.delta ?? "",
        timestamp: incoming.timestamp ?? last.timestamp,
      };
      events[events.length - 1] = merged;
    }
    return true;
  }

  const last = events[events.length - 1];
  if (!last || !isEventType(last, "workflow.node.output.delta")) {
    return false;
  }

  const lastNodeId = last.node_id ?? "";
  const incomingNodeId = incoming.node_id ?? "";

  if ((lastNodeId || incomingNodeId) && lastNodeId !== incomingNodeId) {
    return false;
  }
  if (last.session_id !== incoming.session_id) {
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

  const MAX_DELTA_CHARS = 10_000;
  const mergedDeltaRaw = `${last.delta ?? ""}${delta}`;
  const mergedDelta =
    mergedDeltaRaw.length > MAX_DELTA_CHARS
      ? mergedDeltaRaw.slice(-MAX_DELTA_CHARS)
      : mergedDeltaRaw;
  const merged: WorkflowNodeOutputDeltaEvent = {
    ...last,
    ...incoming,
    delta: mergedDelta,
    timestamp: incoming.timestamp ?? last.timestamp,
  };

  events[events.length - 1] = merged;
  return true;
}

type DisplayEntry =
  | { kind: "event"; event: AnyAgentEvent }
  | { kind: "clarifyTimeline"; groups: ClarifyTaskGroup[]; ts: number };

type CombinedEntry =
  | { kind: "event"; event: AnyAgentEvent; ts: number; order: number }
  | { kind: "clarifyTimeline"; groups: ClarifyTaskGroup[]; ts: number; order: number };

function buildInterleavedEntries(
  displayEntries: DisplayEntry[],
): CombinedEntry[] {
  return displayEntries.map((entry, index) => {
    if (entry.kind === "event") {
      return {
        kind: "event",
        event: entry.event,
        ts: Date.parse(entry.event.timestamp ?? "") || 0,
        order: index,
      };
    }
    return {
      kind: "clarifyTimeline",
      groups: entry.groups,
      ts: entry.ts,
      order: index,
    };
  });
}

function resolveActiveTaskId(events: AnyAgentEvent[]): string | null {
  let activeTaskId: string | null = null;

  events.forEach((event) => {
    if (event.event_type === "workflow.input.received") {
      if (event.agent_level !== "core") {
        return;
      }
      if (typeof event.task_id === "string" && event.task_id.trim()) {
        activeTaskId = event.task_id;
      }
      return;
    }
    if (!activeTaskId) {
      return;
    }
    if (event.agent_level !== "core") {
      return;
    }
    if (event.task_id !== activeTaskId) {
      return;
    }
    if (
      isEventType(event, "workflow.result.final", "workflow.result.cancelled") ||
      isEventType(event, "workflow.node.failed")
    ) {
      activeTaskId = null;
    }
  });

  if (activeTaskId) {
    return activeTaskId;
  }

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const event = events[i];
    if (event.agent_level !== "core") {
      continue;
    }
    const taskId = typeof event.task_id === "string" ? event.task_id.trim() : "";
    if (taskId) {
      return taskId;
    }
  }

  return null;
}

function isClarifyToolEvent(event: AnyAgentEvent): boolean {
  if (!isEventType(event, "workflow.tool.completed")) {
    return false;
  }

  if (event.agent_level && event.agent_level !== "core") {
    return false;
  }

  const toolName =
    "tool_name" in event && typeof event.tool_name === "string"
      ? event.tool_name.toLowerCase()
      : "";
  if (toolName !== "clarify") {
    return false;
  }

  const result =
    "result" in event && typeof event.result === "string" ? event.result : "";
  if (isOrchestratorRetryMessage(result)) {
    return false;
  }

  return true;
}

function buildDisplayEntriesWithClarifyTimeline(
  displayEvents: AnyAgentEvent[],
): DisplayEntry[] {
  const entries: DisplayEntry[] = [];
  const groups: ClarifyTaskGroup[] = [];
  let currentGroup: ClarifyTaskGroup | null = null;
  let timelineTs: number | null = null;
  let timelineStarted = false;

  for (const event of displayEvents) {
    if (isClarifyToolEvent(event)) {
      timelineStarted = true;
      if (timelineTs === null) {
        timelineTs = Date.parse(event.timestamp ?? "") || 0;
      }
      if (currentGroup) {
        groups.push(currentGroup);
      }
      currentGroup = {
        clarifyEvent: event as WorkflowToolCompletedEvent,
        events: [],
      };
      continue;
    }

    const isTerminal = isEventType(
      event,
      "workflow.result.final",
      "workflow.result.cancelled",
    );

    if (timelineStarted && isTerminal) {
      if (currentGroup) {
        groups.push(currentGroup);
        currentGroup = null;
      }
      entries.push({ kind: "clarifyTimeline", groups: [...groups], ts: timelineTs ?? 0 });
      groups.length = 0;
      timelineStarted = false;
      timelineTs = null;
      entries.push({ kind: "event", event });
      continue;
    }

    if (timelineStarted && currentGroup) {
      currentGroup.events.push(event);
      continue;
    }

    entries.push({ kind: "event", event });
  }

  if (timelineStarted) {
    if (currentGroup) {
      groups.push(currentGroup);
    }
    if (groups.length > 0) {
      entries.push({ kind: "clarifyTimeline", groups, ts: timelineTs ?? 0 });
    }
  }

  return entries;
}

function mergeSubagentContext(
  existing: SubagentContext,
  incoming: SubagentContext,
): SubagentContext {
  // Merge stats by taking the latest non-empty value
  // Stats like tokens should come from the final event (workflow.result.final)
  const mergedStats = incoming.stats || existing.stats;

  return {
    ...existing,
    ...incoming,
    preview: incoming.preview ?? existing.preview,
    concurrency: incoming.concurrency ?? existing.concurrency,
    progress: incoming.progress ?? existing.progress,
    stats: mergedStats,
    status: incoming.status ?? existing.status,
    statusTone: incoming.statusTone ?? existing.statusTone,
  };
}

function getSubagentKey(event: AnyAgentEvent): string {
  const parentTaskId =
    "parent_task_id" in event && typeof event.parent_task_id === "string"
      ? event.parent_task_id
      : undefined;

  const taskId =
    "task_id" in event && typeof event.task_id === "string"
      ? event.task_id
      : undefined;

  const subtaskIndex =
    "subtask_index" in event && typeof event.subtask_index === "number"
      ? event.subtask_index
      : undefined;

  const callId =
    "call_id" in event && typeof event.call_id === "string"
      ? event.call_id
      : undefined;

  if (parentTaskId) {
    if (taskId) {
      return `${parentTaskId}:${taskId}`;
    }
    if (callId) {
      return `${parentTaskId}:call:${callId}`;
    }
    if (typeof subtaskIndex === "number") {
      return `${parentTaskId}:subtask:${subtaskIndex}`;
    }
    return `parent:${parentTaskId}`;
  }
  if (taskId) {
    return `task:${taskId}`;
  }
  if (callId) {
    return `call:${callId}`;
  }
  return `task:${event.task_id ?? "unknown"}`;
}

function getSubtaskIndex(event: AnyAgentEvent): number {
  const subtaskIndex =
    "subtask_index" in event && typeof event.subtask_index === "number"
      ? event.subtask_index
      : Number.POSITIVE_INFINITY;
  return subtaskIndex;
}

function isSubagentToolEvent(event: AnyAgentEvent): boolean {
  const toolName =
    "tool_name" in event && typeof event.tool_name === "string"
      ? event.tool_name.toLowerCase()
      : "";
  return toolName === "subagent";
}


function shouldDisplaySubagentEvent(event: AnyAgentEvent): boolean {
  return (
    isEventType(
      event,
      "workflow.tool.completed",
      "workflow.result.final",
      "workflow.result.cancelled",
      "workflow.node.output.summary",
      "workflow.node.failed",
    ) || false
  );
}

function sortSubagentEvents(
  events: AnyAgentEvent[],
  arrivalOrder: WeakMap<AnyAgentEvent, number>,
): AnyAgentEvent[] {
  return [...events].sort((a, b) => {
    const tA = parseEventTimestamp(a) ?? 0;
    const tB = parseEventTimestamp(b) ?? 0;
    if (tA !== tB) return tA - tB;
    const aArrival = arrivalOrder.get(a) ?? 0;
    const bArrival = arrivalOrder.get(b) ?? 0;
    return aArrival - bArrival;
  });
}

function getStableEventKey(event: AnyAgentEvent, index: number): string {
  if (isEventType(event, "workflow.node.output.delta")) {
    const sessionId = typeof event.session_id === "string" ? event.session_id : "";
    const taskId = typeof event.task_id === "string" ? event.task_id : "";
    const parentTaskId =
      "parent_task_id" in event && typeof event.parent_task_id === "string"
        ? event.parent_task_id
        : "";
    const agentLevel = typeof event.agent_level === "string" ? event.agent_level : "";
    const iteration =
      "iteration" in event && typeof event.iteration === "number"
        ? String(event.iteration)
        : "";
    const nodeId = event.node_id ?? "";
    return `delta:${sessionId}:${taskId}:${parentTaskId}:${agentLevel}:${iteration}:${nodeId}`;
  }

  if (isEventType(event, "workflow.result.final")) {
    const sessionId = typeof event.session_id === "string" ? event.session_id : "";
    const taskId = typeof event.task_id === "string" ? event.task_id : "";
    const parentTaskId =
      "parent_task_id" in event && typeof event.parent_task_id === "string"
        ? event.parent_task_id
        : "";
    const agentLevel = typeof event.agent_level === "string" ? event.agent_level : "";
    return `final:${sessionId}:${taskId}:${parentTaskId}:${agentLevel}`;
  }

  return `${event.event_type}-${event.timestamp}-${index}`;
}
