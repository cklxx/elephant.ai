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
  const { displayEvents, subagentThreads } = useMemo(
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

  const combinedEntries = useMemo(
    () => buildInterleavedEntries(displayEntries, subagentThreads),
    [displayEntries, subagentThreads]
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
  /** Anchor event info - identifies where this subagent should be inserted in the timeline */
  anchor?: {
    /** The event that triggered this subagent (e.g., subagent tool call) */
    triggerEvent: AnyAgentEvent;
    /** Position in the original events array */
    originalIndex: number;
  };
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

/**
 * First pass: identify all anchor events (subagent tool calls) and their positions.
 * Returns a map from subagent key -> anchor info
 */
function identifySubagentAnchors(
  events: AnyAgentEvent[],
): Map<string, { triggerEvent: AnyAgentEvent; originalIndex: number }> {
  const anchors = new Map<string, { triggerEvent: AnyAgentEvent; originalIndex: number }>();

  events.forEach((event, index) => {
    // Look for subagent tool calls (started/completed/progress)
    if (!isSubagentToolEvent(event)) return;

    const callId = "call_id" in event && typeof event.call_id === "string" ? event.call_id : undefined;
    const parentTaskId = "parent_task_id" in event && typeof event.parent_task_id === "string" ? event.parent_task_id : undefined;
    const taskId = "task_id" in event && typeof event.task_id === "string" ? event.task_id : undefined;

    // Create anchor key same as getSubagentKey logic
    let anchorKey: string | undefined;
    if (parentTaskId) {
      if (taskId) {
        anchorKey = `${parentTaskId}:${taskId}`;
      } else if (callId) {
        anchorKey = `${parentTaskId}:call:${callId}`;
      }
    } else if (taskId) {
      anchorKey = `task:${taskId}`;
    }

    if (anchorKey && !anchors.has(anchorKey)) {
      anchors.set(anchorKey, { triggerEvent: event, originalIndex: index });
    }
  });

  return anchors;
}

function partitionEvents(
  events: AnyAgentEvent[],
  options: { includeDeltas: boolean; activeTaskId: string | null },
): {
  displayEvents: AnyAgentEvent[];
  subagentThreads: SubagentThread[];
} {
  // First pass: identify all anchor events before processing
  const anchorMap = identifySubagentAnchors(events);

  const displayEvents: AnyAgentEvent[] = [];
  const threads = new Map<string, SubagentThread>();
  const arrivalOrder = new WeakMap<AnyAgentEvent, number>();
  const finalAnswerByThreadKey = new Map<string, string>();
  let arrival = 0;
  const includeDeltas = options.includeDeltas;
  const activeTaskId = options.activeTaskId;

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

    // First: Check if this is a subagent tool call (anchor event)
    // This must be checked BEFORE isSubagentLike, because subagent tool calls
    // may have agent_level === "subagent" but should still serve as anchors
    const isAnchorEvent = isSubagentToolEvent(event);
    if (isAnchorEvent) {
      // Always add subagent tool calls to display events so they appear in timeline
      displayEvents.push(event);
    }

    const isSubagentEvent = isSubagentLike(event);

    if (isSubagentEvent) {
      const key = getSubagentKey(event);
      const context = getSubagentContext(event);
      const subtaskIndex = getSubtaskIndex(event);

      // Look up the anchor for this subagent thread
      const anchor = anchorMap.get(key);

      if (!threads.has(key)) {
        threads.set(key, {
          key,
          groupKey: getSubagentGroupKey(event),
          context,
          events: [],
          subtaskIndex,
          firstSeenAt: eventTs,
          firstArrival: arrival,
          anchor,
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

    // Skip if already added as anchor event above
    if (isAnchorEvent) {
      return;
    }

    if (
      !shouldSkipDuplicateSummaryEvent(event, finalAnswerByThreadKey) &&
      !shouldSkipEvent(event)
    ) {
      displayEvents.push(event);
    }
  });

  const sortedThreads = Array.from(threads.values())
    .map((thread) => ({
      ...thread,
      events: sortSubagentEvents(thread.events, arrivalOrder),
    }))
    .sort((a, b) => {
      // Sort by anchor originalIndex if available
      if (a.anchor && b.anchor) {
        return a.anchor.originalIndex - b.anchor.originalIndex;
      }
      // Threads with anchors come before those without
      if (a.anchor && !b.anchor) return -1;
      if (!a.anchor && b.anchor) return 1;
      // Fallback to subtask index (if both are valid/finite)
      const aHasValidSubtask = Number.isFinite(a.subtaskIndex);
      const bHasValidSubtask = Number.isFinite(b.subtaskIndex);
      if (aHasValidSubtask && bHasValidSubtask && a.subtaskIndex !== b.subtaskIndex) {
        return a.subtaskIndex - b.subtaskIndex;
      }
      // If only one has valid subtask index, it comes first
      if (aHasValidSubtask && !bHasValidSubtask) return -1;
      if (!aHasValidSubtask && bHasValidSubtask) return 1;
      // Final fallback: sort by arrival order (first seen first)
      return a.firstArrival - b.firstArrival;
    });

  return {
    displayEvents,
    subagentThreads: sortedThreads,
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
  };

/**
 * Build interleaved timeline with subagent groups inserted at their anchor positions.
 *
 * Subagent threads are already sorted by partitionEvents based on their anchor's originalIndex.
 * We need to convert display entries to combined entries and insert subagent groups
 * at the positions corresponding to their anchors.
 *
 * Since subagent tool calls may be inside clearify timeline entries, we use a pointer
 * to track which subagent group should be inserted next based on anchor order.
 */
function buildInterleavedEntries(
  displayEntries: DisplayEntry[],
  subagentThreads: SubagentThread[],
): CombinedEntry[] {
  // Group threads by groupKey for rendering
  const groupedThreads = new Map<string, SubagentThread[]>();
  subagentThreads.forEach((thread) => {
    const groupKey = thread.groupKey || thread.key;
    const group = groupedThreads.get(groupKey);
    if (!group) {
      groupedThreads.set(groupKey, [thread]);
    } else {
      group.push(thread);
    }
  });

  // Create array of groups with their earliest anchor index
  const subagentGroups: Array<{
    groupKey: string;
    threads: SubagentThread[];
    anchorOriginalIndex: number;
  }> = [];

  groupedThreads.forEach((threads, groupKey) => {
    // Find earliest anchor among threads in this group
    let earliestAnchorIndex = Number.POSITIVE_INFINITY;
    threads.forEach((t) => {
      if (t.anchor) {
        earliestAnchorIndex = Math.min(earliestAnchorIndex, t.anchor.originalIndex);
      }
    });
    subagentGroups.push({
      groupKey,
      threads,
      anchorOriginalIndex: earliestAnchorIndex,
    });
  });

  // Sort groups by anchor position
  subagentGroups.sort((a, b) => {
    // Groups with anchors come before those without
    if (a.anchorOriginalIndex !== Number.POSITIVE_INFINITY && b.anchorOriginalIndex === Number.POSITIVE_INFINITY) {
      return -1;
    }
    if (a.anchorOriginalIndex === Number.POSITIVE_INFINITY && b.anchorOriginalIndex !== Number.POSITIVE_INFINITY) {
      return 1;
    }
    // Both have anchors or both don't - sort by anchor index or subtask index
    if (a.anchorOriginalIndex !== b.anchorOriginalIndex) {
      return a.anchorOriginalIndex - b.anchorOriginalIndex;
    }
    // Fallback to first thread's subtaskIndex
    const aSubtaskIdx = a.threads[0]?.subtaskIndex ?? Number.POSITIVE_INFINITY;
    const bSubtaskIdx = b.threads[0]?.subtaskIndex ?? Number.POSITIVE_INFINITY;
    return aSubtaskIdx - bSubtaskIdx;
  });

  // Build the result by iterating through displayEntries and inserting subagent groups
  const result: CombinedEntry[] = [];
  let subagentGroupIndex = 0;

  // We need to track the position in the original events array
  // Each display entry may correspond to one or more original events
  // For simplicity, we use the entry index and compare with anchor's desired position

  displayEntries.forEach((entry, displayIndex) => {
    // Add the current entry
    if (entry.kind === "event") {
      result.push({
        kind: "event",
        event: entry.event,
        ts: Date.parse(entry.event.timestamp ?? "") || 0,
        order: displayIndex,
      });
    } else {
      result.push({
        kind: "clearifyTimeline",
        groups: entry.groups,
        ts: entry.ts,
        order: displayIndex,
      });
    }

    // Insert all subagent groups that should appear after this entry
    // A group should be inserted if its anchorOriginalIndex is "covered" by this entry
    // Since we don't have exact mapping, we use a heuristic:
    // Insert groups whose anchor position is roughly at or before this display position
    while (subagentGroupIndex < subagentGroups.length) {
      const group = subagentGroups[subagentGroupIndex];

      // Determine if this group should be inserted now
      let shouldInsert = false;

      if (group.anchorOriginalIndex !== Number.POSITIVE_INFINITY) {
        // Has anchor - insert after we've passed enough entries
        // The anchor is at position X in original events, we insert after entry at position ~X
        // Since displayEntries is a subset, we use displayIndex as approximation
        // But we need to be careful not to insert too early
        shouldInsert = displayIndex >= Math.min(group.anchorOriginalIndex, displayEntries.length - 1);
      } else {
        // No anchor - use timestamp fallback
        const groupTs = group.threads[0]?.firstSeenAt ?? Number.POSITIVE_INFINITY;
        const entryTs = entry.kind === "event"
          ? (Date.parse(entry.event.timestamp ?? "") || 0)
          : entry.ts;
        shouldInsert = groupTs <= entryTs;
      }

      if (!shouldInsert) break;

      // Insert this group
      const groupTs = group.threads[0]?.firstSeenAt ?? (entry.kind === "event"
        ? (Date.parse(entry.event.timestamp ?? "") || 0)
        : entry.ts);
      result.push({
        kind: "subagentGroup",
        groupKey: group.groupKey,
        threads: group.threads,
        ts: groupTs,
        order: displayIndex,
      });

      subagentGroupIndex++;
    }
  });

  // Add remaining groups at the end
  while (subagentGroupIndex < subagentGroups.length) {
    const group = subagentGroups[subagentGroupIndex];
    const lastEntry = displayEntries[displayEntries.length - 1];
    const lastTs = lastEntry
      ? (lastEntry.kind === "event"
        ? (Date.parse(lastEntry.event.timestamp ?? "") || 0)
        : lastEntry.ts)
      : 0;

    result.push({
      kind: "subagentGroup",
      groupKey: group.groupKey,
      threads: group.threads,
      ts: group.threads[0]?.firstSeenAt ?? lastTs,
      order: displayEntries.length,
    });

    subagentGroupIndex++;
  }

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
 * Check if event is a subagent tool event (started/completed/progress).
 */
function isSubagentToolEvent(event: AnyAgentEvent): boolean {
  const toolName = getToolName(event);
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
