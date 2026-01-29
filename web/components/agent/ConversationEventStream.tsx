"use client";

import { useMemo } from "react";
import type {
  AnyAgentEvent,
  WorkflowToolStartedEvent,
} from "@/lib/types";
import { isEventType } from "@/lib/events/matching";
import { ConnectionBanner } from "./ConnectionBanner";
import { LoadingDots } from "@/components/ui/loading-states";
import { EventLine } from "./EventLine";
import { isSubagentLike } from "@/lib/subagent";
import { ToolOutputCard } from "./ToolOutputCard";
import { cn } from "@/lib/utils";

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

export function ConversationEventStream({
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

        {mainStream.map((event, index) => (
          <div
            key={getEventKey(event, index)}
            className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
          >
            <EventLine
              event={event}
              pairedToolStartEvent={resolvePairedToolStart(event)}
            />
          </div>
        ))}

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

// ============================================================================
// Keys
// ============================================================================

function getEventKey(event: AnyAgentEvent, index: number): string {
  const eventId =
    "event_id" in event && typeof event.event_id === "string" ? event.event_id : undefined;
  if (eventId) {
    return `event-${eventId}`;
  }
  const seq = "seq" in event && typeof event.seq === "number" ? event.seq : undefined;
  if (seq !== undefined) {
    return `event-seq-${seq}-${index}`;
  }
  return `event-${event.event_type}-${index}`;
}

interface PartitionResult {
  mainStream: AnyAgentEvent[];
  pendingTools: Map<string, WorkflowToolStartedEvent>;
  resolvePairedToolStart: (event: AnyAgentEvent) => WorkflowToolStartedEvent | undefined;
}

// ============================================================================
// Event Partitioning
// ============================================================================

function partitionEvents(events: AnyAgentEvent[], isRunning: boolean): PartitionResult {
  const mainStream: AnyAgentEvent[] = [];
  const startedTools = new Map<string, WorkflowToolStartedEvent>();
  const completedToolIds = new Set<string>();

  // First pass: track lifecycle states
  events.forEach((event) => {
    if (isSubagentLike(event)) {
      return;
    }
    const sessionId = typeof event.session_id === "string" ? event.session_id : "";

    if (isEventType(event, "workflow.tool.started")) {
      startedTools.set(`${sessionId}:${event.call_id}`, event);
    } else if (isEventType(event, "workflow.tool.completed")) {
      completedToolIds.add(`${sessionId}:${event.call_id}`);
    }
  });

  // Find pending (started but not completed)
  const pendingTools = new Map<string, WorkflowToolStartedEvent>();
  startedTools.forEach((started, key) => {
    if (!completedToolIds.has(key)) pendingTools.set(key, started);
  });

  // Create resolve function
  const resolvePairedToolStart = (event: AnyAgentEvent): WorkflowToolStartedEvent | undefined => {
    if (!isEventType(event, "workflow.tool.completed")) return undefined;
    const callId = event.call_id;
    const sessionId = typeof event.session_id === "string" ? event.session_id : "";
    if (!callId) return undefined;
    return startedTools.get(`${sessionId}:${callId}`);
  };

  // Second pass: build display streams
  events.forEach((event) => {
    if (isSubagentLike(event)) {
      return;
    }

    // Core agent events → main stream
    if (shouldDisplayInMainStream(event, isRunning)) {
      mainStream.push(event);
    }
  });

  return { mainStream, pendingTools, resolvePairedToolStart };
}

// ============================================================================
// Display Rules
// ============================================================================

function shouldDisplayInMainStream(event: AnyAgentEvent, isRunning: boolean): boolean {
  // Always show user input
  if (isEventType(event, "workflow.input.received")) return true;

  // Show AI output (deltas only when running and from core agent)
  if (isEventType(event, "workflow.node.output.delta")) {
    return isRunning && event.agent_level !== "subagent";
  }

  // Show AI summary
  if (isEventType(event, "workflow.node.output.summary")) return true;

  // Show final results
  if (isEventType(event, "workflow.result.final", "workflow.result.cancelled")) return true;

  // Show tool completed (started is merged in)
  if (isEventType(event, "workflow.tool.completed")) return true;

  // Skip internal lifecycle
  if (isEventType(event, "workflow.node.started", "workflow.node.completed")) return false;
  if (isEventType(event, "workflow.tool.started")) return false;

  // Skip diagnostics
  if (event.event_type.startsWith("workflow.diagnostic")) return false;

  return true;
}

// ============================================================================
// Utilities
// ============================================================================

function parseEventTimestamp(event: AnyAgentEvent): number | null {
  const ts = Date.parse(event.timestamp ?? "");
  return Number.isFinite(ts) ? ts : null;
}

function sortEventsBySeq(events: AnyAgentEvent[]): AnyAgentEvent[] {
  const normalized = events.map((event, index) => ({
    event,
    index,
    seq: "seq" in event && typeof event.seq === "number" ? event.seq : null,
    ts: parseEventTimestamp(event) ?? Number.MAX_SAFE_INTEGER,
  }));

  normalized.sort((a, b) => {
    if (a.seq !== null && b.seq !== null) {
      if (a.seq !== b.seq) {
        return a.seq - b.seq;
      }
      return a.index - b.index;
    }
    if (a.seq !== null || b.seq !== null) {
      return a.seq !== null ? -1 : 1;
    }
    if (a.ts !== b.ts) {
      return a.ts - b.ts;
    }
    return a.index - b.index;
  });

  return normalized.map((item) => item.event);
}
