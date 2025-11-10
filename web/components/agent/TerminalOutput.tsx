"use client";

import { useMemo } from "react";
import { AnyAgentEvent } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { IntermediatePanel } from "./IntermediatePanel";
import { useI18n } from "@/lib/i18n";
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
  const { t } = useI18n();

  // Filter events to show only user input and final results
  const filteredEvents = useMemo(() => {
    return events.filter((event) => !shouldSkipEvent(event));
  }, [events]);

  const panelAnchors = useMemo(() => {
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
        events.find((event) => event.event_type === "task_analysis") ??
        events[0];
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
  }, [events]);

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
    <div
      className="console-card space-y-5 px-6 py-5"
      data-testid="conversation-stream"
    >
      <div className="space-y-4" data-testid="conversation-events">
        {filteredEvents.map((event, index) => {
          const key = `${event.event_type}-${event.timestamp}-${index}`;
          const panelEvents = panelAnchors.get(event);
          if (panelEvents) {
            return (
              <div key={key} className="space-y-2">
                <EventLine event={event} />
                <IntermediatePanel events={panelEvents} />
              </div>
            );
          }

          return <EventLine key={key} event={event} />;
        })}
      </div>

      {isConnected && filteredEvents.length > 0 && (
        <div className="flex items-center gap-2 pt-1 text-xs text-muted-foreground">
          <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-foreground" />
          <span>{t("conversation.status.listening")}</span>
        </div>
      )}
    </div>
  );
}

/**
 * Filter out noise events that don't provide meaningful information to users
 * Only show key results and important milestones
 */
function shouldSkipEvent(event: AnyAgentEvent): boolean {
  switch (event.event_type) {
    // Show user input
    case "user_task":
    case "task_analysis":
    // Show task completion
    case "task_complete":
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
