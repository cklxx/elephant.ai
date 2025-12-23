"use client";

import { useMemo } from "react";
import {
  AnyAgentEvent,
  WorkflowToolStartedEvent,
  eventMatches,
} from "@/lib/types";
import type { WorkflowToolCompletedEvent } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { LoadingDots } from "@/components/ui/loading-states";
import {
  EventLine,
  SubagentContext,
  SubagentHeader,
  getSubagentContext,
  isSubagentLike,
} from "./EventLine";
import { ClearifyTimeline, type ClearifyTaskGroup } from "./ClearifyTimeline";

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
  const { displayEvents, subagentThreads } = useMemo(
    () => partitionEvents(events),
    [events],
  );

  const displayEntries = useMemo(
    () => buildDisplayEntriesWithClearifyTimeline(displayEvents),
    [displayEvents],
  );

  const toolStartEventsByCallKey = useMemo(() => {
    const map = new Map<string, WorkflowToolStartedEvent>();
    events.forEach((event) => {
      if (!eventMatches(event, "workflow.tool.started")) {
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
      if (!eventMatches(event, "workflow.tool.completed")) {
        return undefined;
      }
      const callId = (event as any).call_id as string | undefined;
      const sessionId = typeof event.session_id === "string" ? event.session_id : "";
      if (!callId) {
        return undefined;
      }
      return toolStartEventsByCallKey.get(`${sessionId}:${callId}`);
    };
  }, [toolStartEventsByCallKey]);

  const combinedEntries = useMemo(() => {
    type CombinedEntry =
      | { kind: "event"; event: AnyAgentEvent; ts: number; order: number }
      | { kind: "clearifyTimeline"; groups: ClearifyTaskGroup[]; ts: number; order: number }
      | { kind: "subagent"; thread: SubagentThread; ts: number; order: number };

    const entries: CombinedEntry[] = displayEntries.map((entry, idx) => {
      if (entry.kind === "event") {
        return {
          kind: "event",
          event: entry.event,
          ts: Date.parse(entry.event.timestamp ?? "") || 0,
          order: idx,
        };
      }

      return {
        kind: "clearifyTimeline",
        groups: entry.groups,
        ts: entry.ts,
        order: idx,
      };
    });

    subagentThreads.forEach((thread, idx) => {
      const first = thread.events[0];
      entries.push({
        kind: "subagent",
        thread,
        ts: (first && Date.parse(first.timestamp ?? "")) || 0,
        order: displayEntries.length + idx,
      });
    });

    return entries.sort((a, b) => {
      if (a.ts !== b.ts) return a.ts - b.ts;
      return a.order - b.order;
    });
  }, [displayEntries, subagentThreads]);

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
          if (entry.kind === "subagent") {
            return (
              <div
                key={entry.thread.key}
                className="group my-2 -mx-2 px-2 transition-colors rounded-lg hover:bg-muted/10"
                data-testid="subagent-thread"
                data-subagent-key={entry.thread.key}
              >
                <div className="rounded-lg border border-border/40 bg-muted/10 p-2 transition-colors group-hover:bg-muted/20">
                  <div className="mb-2">
                    <SubagentHeader context={entry.thread.context} />
                  </div>
                  <div className="space-y-1">
                    {entry.thread.events.map((ev, i) => (
                      <div
                        key={`${entry.thread.key}-${ev.event_type}-${ev.timestamp}-${i}`}
                        className="transition-colors rounded-md hover:bg-muted/10 -mx-2 px-2"
                      >
                        <EventLine
                          event={ev}
                          showSubagentContext={false}
                          pairedToolStartEvent={resolvePairedToolStart(ev)}
                          variant="nested"
                        />
                      </div>
                    ))}
                  </div>
                </div>
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
          const key = `${event.event_type}-${event.timestamp}-${index}`;

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
            className="mt-4 flex max-w-[fit-content] items-center gap-2 rounded-full border border-border/60 bg-background/70 px-3 py-2 text-muted-foreground"
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
}

function shouldSkipEvent(event: AnyAgentEvent): boolean {
  if (event.agent_level === "subagent") {
    return false;
  }

  if (
    event.event_type === "workflow.input.received" ||
    eventMatches(event, "workflow.result.final", "workflow.result.final") ||
    eventMatches(
      event,
      "workflow.result.cancelled",
      "workflow.result.cancelled",
    ) ||
    eventMatches(event, "workflow.node.failed", "workflow.node.failed") ||
    eventMatches(
      event,
      "workflow.node.output.summary",
      "workflow.node.output.summary",
    ) ||
    eventMatches(event, "workflow.tool.completed", "workflow.tool.completed")
  ) {
    return false;
  }

  return true;
}

function shouldHideClearify(
  event: AnyAgentEvent,
  planTaskCountForSession?: number,
): boolean {
  if (planTaskCountForSession !== 1) {
    return false;
  }

  if (event.agent_level && event.agent_level !== "core") {
    return false;
  }

  return (
    eventMatches(event, "workflow.tool.completed") &&
    getToolName(event) === "clearify"
  );
}

function partitionEvents(events: AnyAgentEvent[]): {
  displayEvents: AnyAgentEvent[];
  subagentThreads: SubagentThread[];
} {
  const displayEvents: AnyAgentEvent[] = [];
  const threads = new Map<string, SubagentThread>();
  const arrivalOrder = new WeakMap<AnyAgentEvent, number>();
  const planTaskCounts = new Map<string, number>();
  const finalAnswerByThreadKey = new Map<string, string>();
  let arrival = 0;

  events.forEach((event) => {
    if (!eventMatches(event, "workflow.result.final", "workflow.result.final")) {
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

    const sessionId =
      typeof event.session_id === "string" && event.session_id.trim()
        ? event.session_id
        : null;

    if (sessionId && isPlanToolEvent(event)) {
      const taskCount = getPlanTaskCount(event);
      if (taskCount !== null) {
        planTaskCounts.set(sessionId, taskCount);
      }
    }

    if (sessionId && shouldHideClearify(event, planTaskCounts.get(sessionId))) {
      return;
    }
    if (isDelegationToolEvent(event)) {
      return;
    }

    const isSubagentEvent = isSubagentLike(event);

    if (isSubagentEvent) {
      const key = getSubagentKey(event);
      const context = getSubagentContext(event);
      const subtaskIndex = getSubtaskIndex(event);

      if (!threads.has(key)) {
        threads.set(key, { key, context, events: [], subtaskIndex });
      }

      const thread = threads.get(key)!;
      thread.context = mergeSubagentContext(thread.context, context);

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
      !eventMatches(
        event,
        "workflow.node.output.delta",
        "workflow.node.output.delta",
        "workflow.node.output.delta",
      ) &&
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
        if (a.subtaskIndex !== b.subtaskIndex) {
          return a.subtaskIndex - b.subtaskIndex;
        }
        return 0;
      }),
  };
}

function maybeMergeDeltaEvent(
  events: AnyAgentEvent[],
  incoming: AnyAgentEvent,
): boolean {
  if (!eventMatches(incoming, "workflow.node.output.delta")) {
    return false;
  }

  const delta = (incoming as any).delta;
  if (typeof delta !== "string" || delta.length === 0) {
    return true;
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
  const mergedDeltaRaw = `${(last as any).delta ?? ""}${delta}`;
  const mergedDelta =
    mergedDeltaRaw.length > MAX_DELTA_CHARS
      ? mergedDeltaRaw.slice(-MAX_DELTA_CHARS)
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

type DisplayEntry =
  | { kind: "event"; event: AnyAgentEvent }
  | { kind: "clearifyTimeline"; groups: ClearifyTaskGroup[]; ts: number };

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

function shouldSkipDuplicateSummaryEvent(
  event: AnyAgentEvent,
  finalAnswerByThreadKey: Map<string, string>,
): boolean {
  if (
    !eventMatches(event, "workflow.node.output.summary", "workflow.node.output.summary")
  ) {
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
  if (!eventMatches(event, "workflow.tool.completed")) {
    return false;
  }

  if (event.agent_level && event.agent_level !== "core") {
    return false;
  }

  return getToolName(event) === "clearify";
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

    const isTerminal = eventMatches(
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
  const resolvedTitle = incoming.title ?? existing.title;
  return {
    ...existing,
    ...incoming,
    title: resolvedTitle,
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

  const subtaskIndex =
    "subtask_index" in event && typeof event.subtask_index === "number"
      ? event.subtask_index
      : undefined;

  if (parentTaskId) {
    if (typeof subtaskIndex === "number") {
      return `${parentTaskId}:${subtaskIndex}`;
    }
    return `parent:${parentTaskId}`;
  }

  if ("call_id" in event && typeof event.call_id === "string") {
    return `call:${event.call_id}`;
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

function shouldDisplaySubagentEvent(event: AnyAgentEvent): boolean {
  return (
    eventMatches(
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
    !eventMatches(
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
      : "tool" in event && typeof (event as any).tool === "string"
        ? (event as any).tool
        : "") || "";

  const normalized = name.trim().toLowerCase();
  return normalized ? normalized : null;
}

function isPlanToolEvent(event: AnyAgentEvent): boolean {
  if (!eventMatches(event, "workflow.tool.completed")) {
    return false;
  }

  if (event.agent_level && event.agent_level !== "core") {
    return false;
  }

  return getToolName(event) === "plan";
}

function getPlanTaskCount(event: AnyAgentEvent): number | null {
  if (!isPlanToolEvent(event)) {
    return null;
  }

  const metadata =
    "metadata" in event && event.metadata && typeof event.metadata === "object"
      ? (event.metadata as Record<string, any>)
      : null;

  const internalPlan = metadata?.internal_plan ?? metadata?.internalPlan;
  const steps =
    internalPlan && Array.isArray(internalPlan.steps)
      ? internalPlan.steps
      : [];

  if (steps.length > 0) {
    return steps.length;
  }

  const branches =
    internalPlan && Array.isArray(internalPlan.branches)
      ? internalPlan.branches
      : [];

  const totalTasks = branches.reduce((count: number, branch: any) => {
    const tasks = Array.isArray(branch?.tasks) ? branch.tasks : [];
    return count + tasks.length;
  }, 0);

  return totalTasks > 0 ? totalTasks : null;
}

function sortSubagentEvents(
  events: AnyAgentEvent[],
  arrivalOrder: WeakMap<AnyAgentEvent, number>,
): AnyAgentEvent[] {
  return [...events].sort((a, b) => {
    const tA = Date.parse(a.timestamp ?? "") || 0;
    const tB = Date.parse(b.timestamp ?? "") || 0;
    if (tA !== tB) return tA - tB;
    const aArrival = arrivalOrder.get(a) ?? 0;
    const bArrival = arrivalOrder.get(b) ?? 0;
    return aArrival - bArrival;
  });
}

function isDelegationToolEvent(event: AnyAgentEvent): boolean {
  return getToolName(event) === "subagent";
}
