"use client";

import { useMemo } from "react";
import { AnyAgentEvent } from "@/lib/types";
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
    () => buildPanelAnchors(displayEvents),
    [displayEvents],
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
            showSubagentContext={index === 0}
          />
        ))}
      </div>
    </div>
  );
}

function buildPanelAnchors(events: AnyAgentEvent[]): WeakMap<AnyAgentEvent, AnyAgentEvent[]> {
  const anchorMap = new WeakMap<AnyAgentEvent, AnyAgentEvent[]>();
  if (events.length === 0) {
    return anchorMap;
  }

  const userTaskIndices: number[] = [];
  events.forEach((event, index) => {
    if (event.event_type === "user_task") {
      userTaskIndices.push(index);
    }
  });

  if (userTaskIndices.length === 0) {
    anchorMap.set(events[0], events);
    return anchorMap;
  }

  userTaskIndices.forEach((startIdx, idx) => {
    const endIdx = userTaskIndices[idx + 1] ?? events.length;
    const segmentEvents = events.slice(startIdx, endIdx);
    const anchorEvent = events[startIdx];
    if (anchorEvent) {
      anchorMap.set(anchorEvent, segmentEvents);
    }
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

  switch (event.event_type) {
    // Show user input
    case "user_task":
    // Show task completion
    case "task_complete":
    case "task_cancelled":
    // Show failures
    case "error":
      return false;
    case "tool_call_start":
    case "tool_call_complete":
    case "tool_call_stream":
    case "think_complete":
    // Skip everything else
    default:
      return true;
  }
}

function partitionEvents(
  events: AnyAgentEvent[],
): { displayEvents: AnyAgentEvent[]; subagentThreads: SubagentThread[] } {
  const displayEvents: AnyAgentEvent[] = [];
  const threadOrder: string[] = [];
  const threads = new Map<string, SubagentThread>();

  events.forEach((event) => {
    const isSubagentEvent =
      event.agent_level === "subagent" ||
      ("is_subtask" in event && Boolean(event.is_subtask));

    if (isSubagentEvent) {
      const key = getSubagentKey(event);
      const context = getSubagentContext(event);

      if (!threads.has(key)) {
        threadOrder.push(key);
        threads.set(key, { key, context, events: [] });
      }

      const thread = threads.get(key)!;
      if (!thread.context.preview && context.preview) {
        thread.context = { ...thread.context, preview: context.preview };
      }
      if (!thread.context.concurrency && context.concurrency) {
        thread.context = { ...thread.context, concurrency: context.concurrency };
      }

      thread.events.push(event);
      return;
    }

    if (event.event_type !== "assistant_message" && !shouldSkipEvent(event)) {
      displayEvents.push(event);
    }
  });

  return {
    displayEvents,
    subagentThreads: threadOrder.map((key) => threads.get(key)!),
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
