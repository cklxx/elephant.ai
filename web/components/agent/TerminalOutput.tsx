"use client";

import { useMemo } from "react";
import { AnyAgentEvent } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { IntermediatePanel } from "./IntermediatePanel";
import { getLanguageLocale, useI18n } from "@/lib/i18n";
import { ToolCallSummary } from "@/lib/eventAggregation";
import { EventLine } from "./EventLine";

interface TerminalOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
  toolSummaries?: ToolCallSummary[];
}

export function TerminalOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
}: TerminalOutputProps) {
  const { t, language } = useI18n();
  const locale = getLanguageLocale(language);

  // Filter events to show only user input and final results
  const filteredEvents = useMemo(() => {
    return events.filter((event) => !shouldSkipEvent(event));
  }, [events]);
  console.log(filteredEvents, events);
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
          // Show IntermediatePanel after user_task event
          if (event.event_type === "task_analysis") {
            return (
              <div key={`${event.event_type}-${index}`} className="space-y-4">
                <EventLine event={event} />
                <IntermediatePanel events={events} />
              </div>
            );
          }

          return (
            <EventLine key={`${event.event_type}-${index}`} event={event} />
          );
        })}
      </div>

      {isConnected && filteredEvents.length > 0 && (
        <div className="flex items-center gap-2 pt-1 text-xs uppercase tracking-[0.24em] text-muted-foreground">
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
