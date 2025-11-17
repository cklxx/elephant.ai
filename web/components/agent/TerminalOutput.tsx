"use client";

import { useMemo } from "react";
import { AnyAgentEvent, AssistantMessageEvent } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { IntermediatePanel } from "./IntermediatePanel";
import { useI18n } from "@/lib/i18n";
import { EventLine } from "./EventLine";
import { MarkdownRenderer } from "@/components/ui/markdown";
import { formatTimestamp } from "./EventLine/formatters";

interface TerminalOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
}

interface AssistantMessageItem {
  id: string;
  timestamp: string;
  content: string;
  final: boolean;
  sourceModel?: string;
}

type StreamItem =
  | { kind: 'event'; event: AnyAgentEvent }
  | { kind: 'assistant'; message: AssistantMessageItem };

export function TerminalOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
}: TerminalOutputProps) {
  const { t } = useI18n();

  const { streamItems, panelAnchors } = useMemo(() => {
    const items: StreamItem[] = [];
    const assistantBuckets = new Map<string, AssistantMessageItem>();
    const nonAssistantEvents = events.filter(
      (event) => event.event_type !== 'assistant_message',
    );
    const anchorMap = buildPanelAnchors(nonAssistantEvents);

    events.forEach((event, index) => {
      if (event.event_type === 'assistant_message') {
        const assistantEvent = event as AssistantMessageEvent;
        const key = `${assistantEvent.task_id ?? 'task'}:${assistantEvent.parent_task_id ?? 'root'}:${assistantEvent.iteration}`;
        let bucket = assistantBuckets.get(key);
        if (!bucket) {
          bucket = {
            id: `${key}:${index}`,
            timestamp: assistantEvent.timestamp,
            content: '',
            final: assistantEvent.final,
            sourceModel: assistantEvent.source_model,
          };
          assistantBuckets.set(key, bucket);
          items.push({ kind: 'assistant', message: bucket });
        }

        if (assistantEvent.delta) {
          bucket.content += assistantEvent.delta;
        }
        bucket.timestamp = assistantEvent.timestamp;
        bucket.final = assistantEvent.final;
        if (assistantEvent.source_model) {
          bucket.sourceModel = assistantEvent.source_model;
        }
      } else if (!shouldSkipEvent(event)) {
        items.push({ kind: 'event', event });
      }
    });

    return { streamItems: items, panelAnchors: anchorMap };
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
    <div className="space-y-5" data-testid="conversation-stream">
      <div className="space-y-4" data-testid="conversation-events">
        {streamItems.map((item, index) => {
          if (item.kind === 'assistant') {
            if (!item.message.content) {
              return null;
            }
            return (
              <AssistantMessageBubble
                key={item.message.id}
                message={item.message}
              />
            );
          }

          const { event } = item;
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

          return (
            <EventLine key={key} event={event} />
          );
        })}
      </div>

      {isConnected && streamItems.length > 0 && (
        <div className="flex items-center gap-2 pt-1 text-xs text-muted-foreground">
          <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-foreground" />
          <span>{t("conversation.status.listening")}</span>
        </div>
      )}
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

function AssistantMessageBubble({
  message,
}: {
  message: AssistantMessageItem;
}) {
  return (
    <div className="console-assistant-message">
      <div className="console-assistant-bubble">
        <div className="console-assistant-meta">
          <span>{formatTimestamp(message.timestamp)}</span>
          {message.sourceModel && (
            <span className="console-assistant-meta-source">
              {' '}
              · {message.sourceModel}
            </span>
          )}
          {!message.final && (
            <span className="console-assistant-meta-streaming" aria-live="polite">
              ···
            </span>
          )}
        </div>
        <MarkdownRenderer
          content={message.content}
          containerClassName="console-assistant-content"
        />
      </div>
    </div>
  );
}

/**
 * Filter out noise events that don't provide meaningful information to users
 * Only show key results and important milestones
 */
function shouldSkipEvent(event: AnyAgentEvent): boolean {
  switch (event.event_type) {
    case "assistant_message":
      return true;
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
