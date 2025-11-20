"use client";

import { useMemo } from "react";
import { AnyAgentEvent } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { IntermediatePanel } from "./IntermediatePanel";
import { EventLine } from "./EventLine";

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
  const panelAnchors = useMemo(
    () => buildPanelAnchors(events),
    [events],
  );

  const displayEvents = useMemo(
    () => buildDisplayEvents(events),
    [events],
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
      <div className="space-y-4" data-testid="conversation-events">
        {displayEvents.map((event, index) => {
          const key = `${event.event_type}-${event.timestamp}-${index}`;
          const panelEvents = panelAnchors.get(event);
          if (panelEvents) {
            return <IntermediatePanel key={key} events={panelEvents} />;
          }

          return <EventLine key={key} event={event} />;
        })}
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
    const anchor =
      events.find((event) => event.event_type === "task_analysis") ?? events[0];
    if (anchor) {
      anchorMap.set(anchor, events);
    }
    return anchorMap;
  }

  userTaskIndices.forEach((startIdx, idx) => {
    const endIdx = userTaskIndices[idx + 1] ?? events.length;
    const segmentEvents = events.slice(startIdx, endIdx);
    const analysisAnchor = segmentEvents.find(
      (event) => event.event_type === "task_analysis",
    );
    const anchorEvent = analysisAnchor ?? events[startIdx];
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
    case "task_analysis":
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

function buildDisplayEvents(
  events: AnyAgentEvent[],
): AnyAgentEvent[] {
  return events.filter((event) => {
    if (event.agent_level === "subagent") {
      return true;
    }

    return event.event_type !== "assistant_message" && !shouldSkipEvent(event);
  });
}
