"use client";

import { useMemo, memo } from "react";
import type {
  AnyAgentEvent,
} from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import { LoadingDots } from "@/components/ui/loading-states";
import {
  EventLine,
  getSubagentContext,
} from "./EventLine";
import { AgentCard } from "./AgentCard";
import { subagentThreadToCardData } from "./AgentCard/utils";
import { ToolOutputCard } from "./ToolOutputCard";
import { cn } from "@/lib/utils";
import {
  sortEventsBySeq,
  partitionEvents,
  isSubagentToolStarted,
  getEventKey,
  parseEventTimestamp,
  type SubagentThread,
} from "./eventStreamUtils";

interface ConversationEventStreamProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
  isRunning?: boolean;
  optimisticMessages?: Array<{ id: string; content: string; timestamp: string }>;
}

function ConversationEventStreamInner({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  isRunning = false,
  optimisticMessages = [],
}: ConversationEventStreamProps) {
  const orderedEvents = useMemo(() => sortEventsBySeq(events), [events]);
  const {
    mainStream,
    subagentGroups,
    pendingTools,
    resolvePairedToolStart,
  } = useMemo(() => partitionEvents(orderedEvents, isRunning), [orderedEvents, isRunning]);

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
        {/* Optimistic messages */}
        {optimisticMessages.map((msg) => (
          <div
            key={`optimistic-${msg.id}`}
            className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2 opacity-70"
            data-testid="optimistic-message"
          >
            <div className="py-2 flex justify-end">
              <div className="flex w-full max-w-[min(36rem,100%)] flex-col items-end gap-2">
                <div className="w-fit max-w-full rounded-2xl border border-border/60 bg-background px-4 py-3 shadow-sm">
                  <div className="text-base text-foreground">
                    <p className="whitespace-pre-wrap leading-normal">{msg.content}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ))}

        {mainStream.map((event, index) => {
          if (isSubagentToolStarted(event)) {
            // Lookup by run_id first (= core agent's run_id = subagent's
            // parent_run_id = groupKey), then fall back to call_id
            const runId = event.run_id;
            const callId = event.call_id;
            const threads =
              (runId ? subagentGroups.get(runId) : undefined) ??
              (callId ? subagentGroups.get(callId) : undefined);
            const fallback: SubagentThread = {
              key: `subagent-${callId ?? index}`,
              groupKey: callId ?? "unknown",
              context: getSubagentContext(event),
              events: [],
              subtaskIndex: 0,
              firstSeenAt: parseEventTimestamp(event),
            };
            const threadList = threads && threads.length > 0 ? threads : [fallback];
            const sortedThreads = [...threadList].sort((a, b) => {
              if (a.subtaskIndex !== b.subtaskIndex) {
                return a.subtaskIndex - b.subtaskIndex;
              }
              const aTs = a.firstSeenAt ?? Number.POSITIVE_INFINITY;
              const bTs = b.firstSeenAt ?? Number.POSITIVE_INFINITY;
              return aTs - bTs;
            });
            return (
              <div
                key={getEventKey(event, index)}
                className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
                data-testid="subagent-thread-group"
              >
                <div className="border-l-2 border-muted">
                  {sortedThreads.map((thread) => (
                    <div
                      key={thread.key}
                      className="mt-2 mb-2"
                      data-testid="subagent-card-container"
                    >
                      <AgentCard
                        data={subagentThreadToCardData(
                          thread.key,
                          thread.context,
                          thread.events,
                          thread.subtaskIndex,
                        )}
                        resolvePairedToolStart={resolvePairedToolStart}
                        className="mx-0 my-0"
                      />
                    </div>
                  ))}
                </div>
              </div>
            );
          }

          return (
            <div
              key={getEventKey(event, index)}
              className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
            >
              <EventLine
                event={event}
                pairedToolStartEvent={resolvePairedToolStart(event)}
              />
            </div>
          );
        })}

        {/* Pending running states */}
        {Array.from(pendingTools.values()).map((tool) => (
          <div
            key={`pending-tool-${tool.call_id}`}
            className={cn(
              "py-1 pl-2 border-primary/10",
              "group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
            )}
            data-testid="pending-tool-call"
          >
            <ToolOutputCard
              toolName={tool.tool_name}
              parameters={tool.arguments}
              timestamp={tool.timestamp}
              callId={tool.call_id}
              status="running"
            />
          </div>
        ))}

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

function arePropsEqual(
  prev: Readonly<ConversationEventStreamProps>,
  next: Readonly<ConversationEventStreamProps>,
): boolean {
  // Scalar props — quick check
  if (
    prev.isConnected !== next.isConnected ||
    prev.isReconnecting !== next.isReconnecting ||
    prev.error !== next.error ||
    prev.reconnectAttempts !== next.reconnectAttempts ||
    prev.onReconnect !== next.onReconnect ||
    prev.isRunning !== next.isRunning
  ) {
    return false;
  }

  // Optimistic messages — reference check
  if (prev.optimisticMessages !== next.optimisticMessages) {
    return false;
  }

  // Events — same reference → skip
  if (prev.events === next.events) {
    return true;
  }

  // Different length → re-render
  if (prev.events.length !== next.events.length) {
    return false;
  }

  // Same length — check if last event is identical
  if (prev.events.length > 0) {
    const prevLast = prev.events[prev.events.length - 1];
    const nextLast = next.events[next.events.length - 1];
    if (prevLast === nextLast) {
      return true;
    }
  }

  // Default: re-render
  return false;
}

export const ConversationEventStream = memo(ConversationEventStreamInner, arePropsEqual);
