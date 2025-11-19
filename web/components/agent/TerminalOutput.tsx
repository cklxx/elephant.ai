"use client";

import { useMemo } from "react";
import { AnyAgentEvent, AssistantMessageEvent } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { IntermediatePanel } from "./IntermediatePanel";
import { EventLine } from "./EventLine";
import { MarkdownRenderer } from "@/components/ui/markdown";
import { formatTimestamp } from "./EventLine/formatters";

interface AggregatedAssistantMessage {
  kind: "assistant_message";
  timestamp: string;
  content: string;
  sourceModel?: string;
  isStreaming: boolean;
}

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
  const nonAssistantEvents = useMemo(
    () => events.filter((event) => event.event_type !== 'assistant_message'),
    [events],
  );

  const panelAnchors = useMemo(
    () => buildPanelAnchors(nonAssistantEvents),
    [nonAssistantEvents],
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
          if (isAggregatedAssistantMessage(event)) {
            return (
              <AssistantMessageBubble
                key={`assistant-${event.timestamp}-${index}`}
                message={event}
              />
            );
          }

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
    if (event.event_type === 'user_task') {
      userTaskIndices.push(index);
    }
  });

  if (userTaskIndices.length === 0) {
    const anchor =
      events.find((event) => event.event_type === 'task_analysis') ?? events[0];
    if (anchor) {
      anchorMap.set(anchor, events);
    }
    return anchorMap;
  }

  userTaskIndices.forEach((startIdx, idx) => {
    const endIdx = userTaskIndices[idx + 1] ?? events.length;
    const segmentEvents = events.slice(startIdx, endIdx);
    const analysisAnchor = segmentEvents.find(
      (event) => event.event_type === 'task_analysis',
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
): (AnyAgentEvent | AggregatedAssistantMessage)[] {
  const display: (AnyAgentEvent | AggregatedAssistantMessage)[] = [];
  let currentAssistant: AggregatedAssistantMessage | null = null;

  events.forEach((event) => {
    if (event.event_type === "assistant_message") {
      currentAssistant = appendAssistantMessage(display, currentAssistant, event);
      if (event.final) {
        currentAssistant = null;
      }
      return;
    }

    currentAssistant = null;
    if (!shouldSkipEvent(event)) {
      display.push(event);
    }
  });

  return display;
}

function appendAssistantMessage(
  display: (AnyAgentEvent | AggregatedAssistantMessage)[],
  current: AggregatedAssistantMessage | null,
  event: AssistantMessageEvent,
): AggregatedAssistantMessage {
  if (!current) {
    const aggregated: AggregatedAssistantMessage = {
      kind: "assistant_message",
      timestamp: event.timestamp,
      content: event.delta ?? "",
      sourceModel: event.source_model,
      isStreaming: !event.final,
    };
    display.push(aggregated);
    return aggregated;
  }

  current.timestamp = event.timestamp;
  current.content = `${current.content}${event.delta ?? ""}`;
  current.sourceModel = event.source_model ?? current.sourceModel;
  current.isStreaming = !event.final;
  return current;
}

function isAggregatedAssistantMessage(
  event: AnyAgentEvent | AggregatedAssistantMessage,
): event is AggregatedAssistantMessage {
  return (event as AggregatedAssistantMessage).kind === "assistant_message";
}

interface AssistantMessageBubbleProps {
  message: AggregatedAssistantMessage;
}

function AssistantMessageBubble({ message }: AssistantMessageBubbleProps) {
  return (
    <div className="console-assistant-message" data-testid="assistant-message">
      <div className="console-assistant-bubble">
        <div className="console-assistant-meta">
          <span>{formatTimestamp(message.timestamp)}</span>
          {message.sourceModel && (
            <span className="console-assistant-meta-source">
              {message.sourceModel}
            </span>
          )}
          {message.isStreaming && (
            <span className="console-assistant-meta-streaming">LIVE</span>
          )}
        </div>
        <div className="console-assistant-content">
          <MarkdownRenderer content={message.content} />
        </div>
      </div>
    </div>
  );
}
