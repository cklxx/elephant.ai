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
import { ClearifyTimeline, type ClearifyTaskGroup } from "./ClearifyTimeline";
import { isOrchestratorRetryMessage } from "@/lib/utils";
import { AgentCard } from "./AgentCard";
import { subagentThreadToCardData } from "./AgentCard/utils";

interface ConversationEventStreamProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
  isRunning?: boolean;
}

export function ConversationEventStream({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  isRunning = false,
}: ConversationEventStreamProps) {
  const activeTaskId = useMemo(() => resolveActiveTaskId(events), [events]);
  const { displayEvents, subagentThreads, anchorMap } = useMemo(
    () =>
      partitionEvents(events, {
        includeDeltas: isRunning,
        activeTaskId,
      }),
    [events, isRunning, activeTaskId],
  );

  const displayEntries = useMemo(
    () => buildDisplayEntriesWithClearifyTimeline(displayEvents),
    [displayEvents],
  );

  const toolStartEventsByCallKey = useMemo(() => {
    const map = new Map<string, WorkflowToolStartedEvent>();
    events.forEach((event) => {
      if (!isEventType(event, "workflow.tool.started")) {
        return;
      }
      const started = event as WorkflowToolStartedEvent;
      const sessionId =
        typeof started.session_id === "string" ? started.session_id : "";
      if (!started.call_id) {
        return;
      }
      map.set(`${sessionId}:${started.call_id}`, started);
    });
    return map;
  }, [events]);

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
      return toolStartEventsByCallKey.get(`${sessionId}:${callId}`);
    };
  }, [toolStartEventsByCallKey]);

  const combinedEntries = useMemo(() => {
    if (process.env.NODE_ENV === "development") {
      console.log("[ConversationEventStream] subagentThreads count:", subagentThreads.length);
      subagentThreads.forEach((thread, idx) => {
        console.log(`[subagentThread ${idx}]`, {
          key: thread.key,
          groupKey: thread.groupKey,
          eventsCount: thread.events.length,
          subtaskIndex: thread.subtaskIndex,
          anchorEventId: thread.anchorEventId,
        });
      });
    }

    return buildInterleavedEntries(displayEntries, subagentThreads, anchorMap);
  }, [displayEntries, subagentThreads, anchorMap]);

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
        {combinedEntries.map((entry, index) => {
          if (entry.kind === "subagentGroup") {
            return (
              <div
                key={`subagent-group-${entry.groupKey}`}
                className="-mx-2 px-2 my-2 flex flex-row gap-3 overflow-y-scroll"
                data-testid="subagent-thread-group"
              >
                {entry.threads.map((thread) => {
                  const cardData = subagentThreadToCardData(
                    thread.key,
                    thread.context,
                    thread.events,
                    thread.subtaskIndex,
                  );

                  return (
                    <AgentCard
                      key={thread.key}
                      data={cardData}
                      resolvePairedToolStart={resolvePairedToolStart}
                      className="mx-0 my-0"
                    />
                  );
                })}
              </div>
            );
          }

          if (entry.kind === "clearifyTimeline") {
            return (
              <div
                key={`clearify-timeline-${entry.ts}-${index}`}
                className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
                data-testid="clearify-timeline-entry"
              >
                <ClearifyTimeline
                  groups={entry.groups}
                  isRunning={isRunning}
                  resolvePairedToolStart={resolvePairedToolStart}
                />
              </div>
            );
          }

          const event = entry.event;
          const key = getStableEventKey(event, index);

          return (
            <div
              key={key}
              className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
            >
              <EventLine
                event={event}
                pairedToolStartEvent={resolvePairedToolStart(event)}
              />
            </div>
          );
        })}
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
  groupKey: string;
  context: SubagentContext;
  events: AnyAgentEvent[];
  subtaskIndex: number;
  firstSeenAt: number | null;
  firstArrival: number;
  /** Anchor event key - identifies the parent tool call that triggered this subagent */
  anchorEventId?: string;
  /** Timestamp of the anchor event for fallback sorting */
  anchorTimestamp?: number;
}

function parseEventTimestamp(event: AnyAgentEvent): number | null {
  const ts = Date.parse(event.timestamp ?? "");
  return Number.isFinite(ts) ? ts : null;
}

function shouldSkipEvent(event: AnyAgentEvent): boolean {
  if (event.agent_level === "subagent") {
    return false;
  }

  if (
    event.event_type === "workflow.input.received" ||
    isEventType(event, "workflow.result.final") ||
    isEventType(
      event,
      "workflow.result.cancelled",
    ) ||
    isEventType(event, "workflow.node.failed") ||
    isEventType(
      event,
      "workflow.node.output.summary",
    ) ||
    isEventType(event, "workflow.tool.completed")
  ) {
    return false;
  }

  return true;
}

function partitionEvents(
  events: AnyAgentEvent[],
  options: { includeDeltas: boolean; activeTaskId: string | null },
): {
  displayEvents: AnyAgentEvent[];
  subagentThreads: SubagentThread[];
  anchorMap: Map<string, { timestamp: number | null; eventIndex: number }>;
} {
  const displayEvents: AnyAgentEvent[] = [];
  const threads = new Map<string, SubagentThread>();
  const arrivalOrder = new WeakMap<AnyAgentEvent, number>();
  const finalAnswerByThreadKey = new Map<string, string>();
  let arrival = 0;
  const includeDeltas = options.includeDeltas;
  const activeTaskId = options.activeTaskId;

  // Build anchor map from all events before partitioning
  const anchorMap = buildAnchorMap(events);

  events.forEach((event) => {
    if (!isEventType(event, "workflow.result.final")) {
      return;
    }
    const key = getThreadKey(event);
    if (!key) {
      return;
    }
    const finalAnswer =
      "final_answer" in event && typeof event.final_answer === "string"
        ? event.final_answer
        : "";
    if (!finalAnswer.trim()) {
      return;
    }
    finalAnswerByThreadKey.set(key, normalizeComparableText(finalAnswer));
  });

  events.forEach((event) => {
    arrival += 1;
    arrivalOrder.set(event, arrival);

    const eventTs = parseEventTimestamp(event);
    const sessionId =
      typeof event.session_id === "string" && event.session_id.trim()
        ? event.session_id
        : null;

    if (isDelegationToolEvent(event)) {
      return;
    }

    const isSubagentEvent = isSubagentLike(event);

    if (isSubagentEvent) {
      const key = getSubagentKey(event);
      const context = getSubagentContext(event);
      const subtaskIndex = getSubtaskIndex(event);
      const anchorId = getSubagentAnchorId(event);
      const anchorInfo = anchorId ? anchorMap.get(anchorId) : undefined;

      if (!threads.has(key)) {
        threads.set(key, {
          key,
          groupKey: getSubagentGroupKey(event),
          context,
          events: [],
          subtaskIndex,
          firstSeenAt: eventTs,
          firstArrival: arrival,
          anchorEventId: anchorId,
          anchorTimestamp: anchorInfo?.timestamp ?? undefined,
        });
      }

      const thread = threads.get(key)!;
      thread.context = mergeSubagentContext(thread.context, context);
      if (eventTs !== null) {
        if (thread.firstSeenAt === null || eventTs < thread.firstSeenAt) {
          thread.firstSeenAt = eventTs;
        }
      }
      if (arrival < thread.firstArrival) {
        thread.firstArrival = arrival;
      }

      if (!shouldDisplaySubagentEvent(event)) {
        return;
      }
      if (shouldSkipDuplicateSummaryEvent(event, finalAnswerByThreadKey)) {
        return;
      }
      thread.events.push(event);
      return;
    }

    if (
      isEventType(event, "workflow.node.output.delta")
    ) {
      if (
        includeDeltas &&
        !isSubagentEvent &&
        (activeTaskId === null ||
          (typeof event.task_id === "string" && event.task_id === activeTaskId))
      ) {
        if (!maybeMergeDeltaEvent(displayEvents, event)) {
          displayEvents.push(event);
        }
      }
      return;
    }

    if (
      !shouldSkipDuplicateSummaryEvent(event, finalAnswerByThreadKey) &&
      !shouldSkipEvent(event)
    ) {
      displayEvents.push(event);
    }
  });

  return {
    displayEvents,
    subagentThreads: Array.from(threads.values())
      .map((thread) => ({
        ...thread,
        events: sortSubagentEvents(thread.events, arrivalOrder),
      }))
      .sort((a, b) => {
        // Sort by anchor event index if available
        if (a.anchorEventId && b.anchorEventId) {
          const aAnchor = anchorMap.get(a.anchorEventId);
          const bAnchor = anchorMap.get(b.anchorEventId);
          if (aAnchor && bAnchor) {
            return aAnchor.eventIndex - bAnchor.eventIndex;
          }
        }
        // Fallback to subtask index
        if (a.subtaskIndex !== b.subtaskIndex) {
          return a.subtaskIndex - b.subtaskIndex;
        }
        return 0;
      }),
    anchorMap,
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
  const incomingNodeId =
    incoming.node_id ?? "";

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
  | { kind: "clearifyTimeline"; groups: ClearifyTaskGroup[]; ts: number };

type CombinedEntry =
  | { kind: "event"; event: AnyAgentEvent; ts: number; order: number }
  | { kind: "clearifyTimeline"; groups: ClearifyTaskGroup[]; ts: number; order: number }
  | {
    kind: "subagentGroup";
    groupKey: string;
    threads: SubagentThread[];
    ts: number;
    order: number;
    anchorEventId?: string;
  };

/**
 * Build interleaved timeline with subagent groups anchored to their trigger events.
 *
 * This function creates a unified timeline where subagent groups are inserted
 * immediately after their anchor events (the tool calls that triggered them),
 * rather than being sorted purely by timestamp.
 */
function buildInterleavedEntries(
  displayEntries: DisplayEntry[],
  subagentThreads: SubagentThread[],
  anchorMap: Map<string, { timestamp: number | null; eventIndex: number }>,
): CombinedEntry[] {
  // Group subagent threads by their groupKey
  const groupedSubagents = new Map<
    string,
    { groupKey: string; threads: SubagentThread[]; anchorEventId?: string; anchorIndex: number }
  >();

  subagentThreads.forEach((thread) => {
    const groupKey = thread.groupKey || thread.key;
    const group = groupedSubagents.get(groupKey);

    // Determine anchor index for positioning
    let anchorIndex = Number.POSITIVE_INFINITY;
    if (thread.anchorEventId) {
      const anchorInfo = anchorMap.get(thread.anchorEventId);
      if (anchorInfo) {
        anchorIndex = anchorInfo.eventIndex;
      }
    }

    if (!group) {
      groupedSubagents.set(groupKey, {
        groupKey,
        threads: [thread],
        anchorEventId: thread.anchorEventId,
        anchorIndex,
      });
      return;
    }

    group.threads.push(thread);
    // Use the earliest anchor index among threads in the group
    if (anchorIndex < group.anchorIndex) {
      group.anchorIndex = anchorIndex;
      group.anchorEventId = thread.anchorEventId;
    }
  });

  // Convert display entries to combined entries with their original index
  const baseEntries: Array<
    | { kind: "event"; event: AnyAgentEvent; ts: number; order: number; originalIndex: number }
    | { kind: "clearifyTimeline"; groups: ClearifyTaskGroup[]; ts: number; order: number; originalIndex: number }
  > = displayEntries.map((entry, idx) => {
    if (entry.kind === "event") {
      return {
        kind: "event",
        event: entry.event,
        ts: Date.parse(entry.event.timestamp ?? "") || 0,
        order: idx,
        originalIndex: idx,
      };
    }
    return {
      kind: "clearifyTimeline",
      groups: entry.groups,
      ts: entry.ts,
      order: idx,
      originalIndex: idx,
    };
  });

  // Insert subagent groups at their anchor positions
  const result: CombinedEntry[] = [];
  const insertedGroups = new Set<string>();

  baseEntries.forEach((entry, currentIndex) => {
    // Insert any subagent groups that should appear after this entry
    const groupsToInsert: Array<{ groupKey: string; threads: SubagentThread[]; anchorIndex: number }> = [];

    groupedSubagents.forEach((group, groupKey) => {
      if (insertedGroups.has(groupKey)) return;

      // Determine if this group should be inserted after current entry
      let shouldInsert = false;

      if (group.anchorIndex !== Number.POSITIVE_INFINITY) {
        // Has valid anchor - insert after the anchor event
        shouldInsert = group.anchorIndex <= currentIndex;
      } else {
        // No anchor - use timestamp comparison as fallback
        const groupTs = group.threads[0]?.firstSeenAt ?? Number.POSITIVE_INFINITY;
        shouldInsert = groupTs <= entry.ts;
      }

      if (shouldInsert) {
        groupsToInsert.push({
          groupKey: group.groupKey,
          threads: group.threads,
          anchorIndex: group.anchorIndex,
        });
      }
    });

    // Sort groups by anchor index, then by subtask index for stable ordering
    groupsToInsert.sort((a, b) => {
      if (a.anchorIndex !== b.anchorIndex) {
        return a.anchorIndex - b.anchorIndex;
      }
      // Within same anchor, sort by first thread's subtaskIndex
      const aSubtaskIdx = a.threads[0]?.subtaskIndex ?? Number.POSITIVE_INFINITY;
      const bSubtaskIdx = b.threads[0]?.subtaskIndex ?? Number.POSITIVE_INFINITY;
      return aSubtaskIdx - bSubtaskIdx;
    });

    groupsToInsert.forEach((group) => {
      if (insertedGroups.has(group.groupKey)) return;
      insertedGroups.add(group.groupKey);

      const groupTs = group.threads[0]?.firstSeenAt ?? entry.ts;
      result.push({
        kind: "subagentGroup",
        groupKey: group.groupKey,
        threads: group.threads,
        ts: groupTs,
        order: currentIndex,
        anchorEventId: group.threads[0]?.anchorEventId,
      });
    });

    // Add the current entry
    if (entry.kind === "event") {
      result.push({
        kind: "event",
        event: entry.event,
        ts: entry.ts,
        order: entry.order,
      });
    } else {
      result.push({
        kind: "clearifyTimeline",
        groups: entry.groups,
        ts: entry.ts,
        order: entry.order,
      });
    }
  });

  // Insert any remaining groups at the end
  groupedSubagents.forEach((group, groupKey) => {
    if (insertedGroups.has(groupKey)) return;

    const lastEntry = baseEntries[baseEntries.length - 1];
    const lastTs = lastEntry?.ts ?? 0;

    result.push({
      kind: "subagentGroup",
      groupKey: group.groupKey,
      threads: group.threads,
      ts: group.threads[0]?.firstSeenAt ?? lastTs,
      order: baseEntries.length,
      anchorEventId: group.threads[0]?.anchorEventId,
    });
  });

  return result;
}

function normalizeComparableText(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

function getThreadKey(event: AnyAgentEvent): string | null {
  const sessionId = typeof event.session_id === "string" ? event.session_id : "";
  const taskId = typeof event.task_id === "string" ? event.task_id : "";

  if (isSubagentLike(event)) {
    return `subagent:${sessionId}:${getSubagentKey(event)}`;
  }

  if (taskId) {
    return `core:${sessionId}:${taskId}`;
  }

  if (sessionId) {
    return `core:${sessionId}`;
  }

  return null;
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

function shouldSkipDuplicateSummaryEvent(
  event: AnyAgentEvent,
  finalAnswerByThreadKey: Map<string, string>,
): boolean {
  if (!isEventType(event, "workflow.node.output.summary")) {
    return false;
  }

  if (!isSubagentLike(event)) {
    return false;
  }

  const key = getThreadKey(event);
  if (!key) {
    return false;
  }

  const finalAnswer = finalAnswerByThreadKey.get(key);
  if (!finalAnswer) {
    return false;
  }

  const content =
    "content" in event && typeof event.content === "string" ? event.content : "";
  if (!content.trim()) {
    return false;
  }

  return normalizeComparableText(content) === finalAnswer;
}

function isClearifyToolEvent(event: AnyAgentEvent): boolean {
  if (!isEventType(event, "workflow.tool.completed")) {
    return false;
  }

  if (event.agent_level && event.agent_level !== "core") {
    return false;
  }

  if (getToolName(event) !== "clearify") {
    return false;
  }

  const result =
    "result" in event && typeof event.result === "string" ? event.result : "";
  if (isOrchestratorRetryMessage(result)) {
    return false;
  }

  return true;
}

function buildDisplayEntriesWithClearifyTimeline(
  displayEvents: AnyAgentEvent[],
): DisplayEntry[] {
  const entries: DisplayEntry[] = [];
  const groups: ClearifyTaskGroup[] = [];
  let currentGroup: ClearifyTaskGroup | null = null;
  let timelineTs: number | null = null;
  let timelineStarted = false;

  for (const event of displayEvents) {
    if (isClearifyToolEvent(event)) {
      timelineStarted = true;
      if (timelineTs === null) {
        timelineTs = Date.parse(event.timestamp ?? "") || 0;
      }
      if (currentGroup) {
        groups.push(currentGroup);
      }
      currentGroup = {
        clearifyEvent: event as WorkflowToolCompletedEvent,
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
      entries.push({ kind: "clearifyTimeline", groups: [...groups], ts: timelineTs ?? 0 });
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
      entries.push({ kind: "clearifyTimeline", groups, ts: timelineTs ?? 0 });
    }
  }

  return entries;
}

function mergeSubagentContext(
  existing: SubagentContext,
  incoming: SubagentContext,
): SubagentContext {
  return {
    ...existing,
    ...incoming,
    preview: incoming.preview ?? existing.preview,
    concurrency: incoming.concurrency ?? existing.concurrency,
    progress: incoming.progress ?? existing.progress,
    stats: incoming.stats ?? existing.stats,
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

function getSubagentGroupKey(event: AnyAgentEvent): string {
  const sessionId = typeof event.session_id === "string" ? event.session_id : "";
  const parentTaskId =
    "parent_task_id" in event && typeof event.parent_task_id === "string"
      ? event.parent_task_id
      : undefined;
  if (parentTaskId) {
    return `parent:${sessionId}:${parentTaskId}`;
  }

  const taskId =
    "task_id" in event && typeof event.task_id === "string"
      ? event.task_id
      : undefined;
  if (taskId) {
    return `task:${sessionId}:${taskId}`;
  }

  return `subagent:${sessionId}:${getSubagentKey(event)}`;
}

function getSubtaskIndex(event: AnyAgentEvent): number {
  const subtaskIndex =
    "subtask_index" in event && typeof event.subtask_index === "number"
      ? event.subtask_index
      : Number.POSITIVE_INFINITY;
  return subtaskIndex;
}

/**
 * Extract anchor event identifier from subagent event.
 * Anchor identifies the parent tool call that triggered this subagent.
 */
function getSubagentAnchorId(event: AnyAgentEvent): string | undefined {
  // Primary: use call_id if it references a subagent delegation
  const callId =
    "call_id" in event && typeof event.call_id === "string"
      ? event.call_id
      : undefined;
  if (callId?.startsWith("subagent")) {
    return `call:${callId}`;
  }

  // Secondary: use parent_task_id + task_id combination
  const parentTaskId =
    "parent_task_id" in event && typeof event.parent_task_id === "string"
      ? event.parent_task_id
      : undefined;
  const taskId =
    "task_id" in event && typeof event.task_id === "string"
      ? event.task_id
      : undefined;

  if (parentTaskId && taskId) {
    return `task:${parentTaskId}:${taskId}`;
  }

  // Tertiary: use subtask_index if available
  const subtaskIndex =
    "subtask_index" in event && typeof event.subtask_index === "number"
      ? event.subtask_index
      : undefined;
  if (parentTaskId && typeof subtaskIndex === "number") {
    return `subtask:${parentTaskId}:${subtaskIndex}`;
  }

  return undefined;
}

/**
 * Build a lookup map for anchor events from the main event stream.
 * Returns a map from anchorId -> { timestamp, eventIndex }
 */
function buildAnchorMap(
  events: AnyAgentEvent[],
): Map<string, { timestamp: number | null; eventIndex: number }> {
  const anchorMap = new Map<string, { timestamp: number | null; eventIndex: number }>();

  events.forEach((event, index) => {
    // Subagent delegation tool calls create anchors
    if (isDelegationToolEvent(event)) {
      const callId =
        "call_id" in event && typeof event.call_id === "string"
          ? event.call_id
          : undefined;
      if (callId) {
        const anchorId = `call:${callId}`;
        const ts = parseEventTimestamp(event);
        anchorMap.set(anchorId, { timestamp: ts, eventIndex: index });
      }
      return;
    }

    // Also track task creation events as potential anchors
    const parentTaskId =
      "parent_task_id" in event && typeof event.parent_task_id === "string"
        ? event.parent_task_id
        : undefined;
    const taskId =
      "task_id" in event && typeof event.task_id === "string" ? event.task_id : undefined;

    if (parentTaskId && taskId && event.agent_level === "subagent") {
      const anchorId = `task:${parentTaskId}:${taskId}`;
      if (!anchorMap.has(anchorId)) {
        const ts = parseEventTimestamp(event);
        anchorMap.set(anchorId, { timestamp: ts, eventIndex: index });
      }
    }
  });

  return anchorMap;
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

function getToolName(event: AnyAgentEvent): string | null {
  if (
    !isEventType(
      event,
      "workflow.tool.started",
      "workflow.tool.progress",
      "workflow.tool.completed",
    )
  ) {
    return null;
  }

  const name =
    ("tool_name" in event && typeof event.tool_name === "string"
      ? event.tool_name
      : "tool" in event &&
        typeof (event as Record<string, unknown>).tool === "string"
        ? String((event as Record<string, unknown>).tool)
        : "") || "";

  const normalized = name.trim().toLowerCase();
  return normalized ? normalized : null;
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

function isDelegationToolEvent(event: AnyAgentEvent): boolean {
  return getToolName(event) === "subagent";
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
