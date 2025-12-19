"use client";

import { useMemo } from "react";
import { AnyAgentEvent, eventMatches } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { LoadingDots } from "@/components/ui/loading-states";
import {
  EventLine,
  SubagentContext,
  SubagentHeader,
  getSubagentContext,
  isSubagentLike,
} from "./EventLine";

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

  const combinedEntries = useMemo(() => {
    type CombinedEntry =
      | { kind: "event"; event: AnyAgentEvent; ts: number; order: number }
      | { kind: "subagent"; thread: SubagentThread; ts: number; order: number };

    const entries: CombinedEntry[] = displayEvents.map((event, idx) => ({
      kind: "event",
      event,
      ts: Date.parse(event.timestamp ?? "") || 0,
      order: idx,
    }));

    subagentThreads.forEach((thread, idx) => {
      const first = thread.events[0];
      entries.push({
        kind: "subagent",
        thread,
        ts: (first && Date.parse(first.timestamp ?? "")) || 0,
        order: idx,
      });
    });

    return entries.sort((a, b) => {
      if (a.ts !== b.ts) return a.ts - b.ts;
      return a.order - b.order;
    });
  }, [displayEvents, subagentThreads]);

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
                className="pl-4 ml-2 border-l-2 border-primary/10 my-2"
                data-testid="subagent-thread"
                data-subagent-key={entry.thread.key}
              >
                <div className="mb-2">
                  <SubagentHeader context={entry.thread.context} />
                </div>
                <div className="flex flex-col">
                  {entry.thread.events.map((ev, i) => (
                    <EventLine
                      key={`${entry.thread.key}-${ev.event_type}-${ev.timestamp}-${i}`}
                      event={ev}
                      showSubagentContext={false}
                    />
                  ))}
                </div>
              </div>
            );
          }

          const event = entry.event;
          const key = `${event.event_type}-${event.timestamp}-${index}`;

          return (
            <div
              key={key}
              className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2 py-0.5"
            >
              <EventLine event={event} />
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
    eventMatches(event, "workflow.result.cancelled", "workflow.result.cancelled") ||
    eventMatches(event, "workflow.node.failed", "workflow.node.failed") ||
    eventMatches(event, "workflow.node.output.summary", "workflow.node.output.summary") ||
    eventMatches(event, "workflow.tool.completed", "workflow.tool.completed")
  ) {
    return false;
  }

  return true;
}

function partitionEvents(
  events: AnyAgentEvent[],
): { displayEvents: AnyAgentEvent[]; subagentThreads: SubagentThread[] } {
  const displayEvents: AnyAgentEvent[] = [];
  const threads = new Map<string, SubagentThread>();
  const arrivalOrder = new WeakMap<AnyAgentEvent, number>();
  let arrival = 0;

  events.forEach((event) => {
    arrival += 1;
    arrivalOrder.set(event, arrival);
    if (isDelegationToolEvent(event)) {
      return;
    }

    const isSubagentEvent = isSubagentLike(event);

    if (isSubagentEvent) {
      if (!shouldDisplaySubagentEvent(event)) {
        return;
      }
      const key = getSubagentKey(event);
      const context = getSubagentContext(event);
      const subtaskIndex = getSubtaskIndex(event);

      if (!threads.has(key)) {
        threads.set(key, { key, context, events: [], subtaskIndex });
      }

      const thread = threads.get(key)!;
      thread.context = mergeSubagentContext(thread.context, context);
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
      "workflow.subflow.progress",
      "workflow.subflow.completed",
      "workflow.tool.started",
      "workflow.tool.progress",
      "workflow.tool.completed",
      "workflow.result.final",
      "workflow.result.cancelled",
      "workflow.node.output.delta",
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
    const tA = Date.parse(a.timestamp ?? "") || 0;
    const tB = Date.parse(b.timestamp ?? "") || 0;
    if (tA !== tB) return tA - tB;
    const aArrival = arrivalOrder.get(a) ?? 0;
    const bArrival = arrivalOrder.get(b) ?? 0;
    return aArrival - bArrival;
  });
}

function isDelegationToolEvent(event: AnyAgentEvent): boolean {
  if (
    !eventMatches(
      event,
      "workflow.tool.started",
      "workflow.tool.progress",
      "workflow.tool.completed",
    )
  ) {
    return false;
  }

  const name =
    ("tool_name" in event &&
      typeof (event as any).tool_name === "string" &&
      (event as any).tool_name) ||
    ("tool" in event &&
      typeof (event as any).tool === "string" &&
      (event as any).tool) ||
    "";
  return name.trim().toLowerCase() === "subagent";
}
