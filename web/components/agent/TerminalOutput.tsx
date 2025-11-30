"use client";

import { useMemo } from "react";
import { AnyAgentEvent, eventMatches } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { IntermediatePanel } from "./IntermediatePanel";
import {
  EventLine,
  SubagentContext,
  SubagentHeader,
  getSubagentContext,
} from "./EventLine";

interface TerminalOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
}

export function TerminalOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
}: TerminalOutputProps) {
  const { displayEvents, subagentThreads } = useMemo(
    () => partitionEvents(events),
    [events],
  );

  const panelAnchors = useMemo(
    () => buildPanelAnchors(events, displayEvents),
    [events, displayEvents],
  );

  // Show connection banner if disconnected
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
    <div className="space-y-5" data-testid="conversation-stream">
      {subagentThreads.length > 0 && (
        <div className="space-y-3" data-testid="subagent-aggregate-panel">
          {subagentThreads.map((thread) => (
            <SubagentAggregate key={thread.key} thread={thread} />
          ))}
        </div>
      )}
      <div className="space-y-4" data-testid="conversation-events">
        {displayEvents.map((event, index) => {
          const key = `${event.event_type}-${event.timestamp}-${index}`;
          const panelEvents = panelAnchors.get(event);
          if (panelEvents) {
            return (
              <div key={key} className="space-y-3">
                <EventLine event={event} />
                <IntermediatePanel events={panelEvents} />
              </div>
            );
          }

          return <EventLine key={key} event={event} />;
        })}
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

function SubagentAggregate({ thread }: { thread: SubagentThread }) {
  return (
    <div
      className="space-y-2 rounded-lg border border-primary/30 bg-primary/5 p-3"
      data-testid="subagent-aggregate"
    >
      <SubagentHeader context={thread.context} />
      <div className="space-y-2">
        {thread.events.map((event, index) => (
          <EventLine
            key={`${thread.key}-${event.event_type}-${event.timestamp}-${index}`}
            event={event}
            showSubagentContext={false}
          />
        ))}
      </div>
    </div>
  );
}

function buildPanelAnchors(
  events: AnyAgentEvent[],
  anchorEvents: AnyAgentEvent[],
): WeakMap<AnyAgentEvent, AnyAgentEvent[]> {
  const anchorMap = new WeakMap<AnyAgentEvent, AnyAgentEvent[]>();
  if (events.length === 0 || anchorEvents.length === 0) {
    return anchorMap;
  }

  const eventIndexLookup = new WeakMap<AnyAgentEvent, number>();
  events.forEach((event, index) => {
    eventIndexLookup.set(event, index);
  });

  const sortedAnchors = anchorEvents
    .map((event) => {
      const index = eventIndexLookup.get(event);
      if (typeof index !== "number") {
        return null;
      }
      return { event, index };
    })
    .filter((anchor): anchor is { event: AnyAgentEvent; index: number } =>
      Boolean(anchor),
    )
    .sort((a, b) => a.index - b.index);

  if (sortedAnchors.length === 0) {
    return anchorMap;
  }

  sortedAnchors.forEach(({ event, index }, idx) => {
    const endIdx = sortedAnchors[idx + 1]?.index ?? events.length;
    const segmentEvents = events.slice(index, endIdx);
    anchorMap.set(event, segmentEvents);
  });

  return anchorMap;
}

/**
 * Filter out noise events that don't provide meaningful information to users
 * Only show key results and important milestones
 */
function shouldSkipEvent(event: AnyAgentEvent): boolean {
  if (event.agent_level === "subagent") {
    return false;
  }

  if (
    event.event_type === "workflow.input.received" ||
    eventMatches(event, "workflow.result.final", "workflow.result.final") ||
    eventMatches(event, "workflow.result.cancelled", "workflow.result.cancelled") ||
    eventMatches(event, "workflow.node.failed", "workflow.node.failed")
  ) {
    return false;
  }

  if (
    eventMatches(
      event,
      "workflow.tool.started",
      "workflow.tool.completed",
      "workflow.tool.progress",
      "workflow.tool.started",
      "workflow.tool.completed",
      "workflow.tool.progress",
      "workflow.node.output.summary",
      "workflow.node.output.summary",
    )
  ) {
    return true;
  }

  return true;
}

function partitionEvents(
  events: AnyAgentEvent[],
): { displayEvents: AnyAgentEvent[]; subagentThreads: SubagentThread[] } {
  const displayEvents: AnyAgentEvent[] = [];
  const threads = new Map<string, SubagentThread>();
  let arrival = 0;

  events.forEach((event) => {
    arrival += 1;
    const isSubagentEvent =
      event.agent_level === "subagent" ||
      ("is_subtask" in event && Boolean(event.is_subtask));

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
      thread.events.push({ ...event, __arrival: arrival } as AnyAgentEvent);
      return;
    }

    if (
      !eventMatches(event, "workflow.node.output.delta", "workflow.node.output.delta", "workflow.node.output.delta") &&
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
        events: sortSubagentEvents(thread.events),
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
  const type = event.event_type;
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

function sortSubagentEvents(events: AnyAgentEvent[]): AnyAgentEvent[] {
  return [...events].sort((a, b) => {
    const tA = Date.parse(a.timestamp ?? "") || 0;
    const tB = Date.parse(b.timestamp ?? "") || 0;
    if (tA !== tB) return tA - tB;
    const aArrival = (a as any).__arrival ?? 0;
    const bArrival = (b as any).__arrival ?? 0;
    return aArrival - bArrival;
  });
}
