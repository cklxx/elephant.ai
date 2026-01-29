"use client";

import { useMemo } from "react";
import type {
  AnyAgentEvent,
  WorkflowToolStartedEvent,
} from "@/lib/types";
import { isEventType } from "@/lib/events/matching";
import { ConnectionBanner } from "./ConnectionBanner";
import { LoadingDots } from "@/components/ui/loading-states";
import {
  EventLine,
  SubagentContext,
  getSubagentContext,
} from "./EventLine";
import { isSubagentLike } from "@/lib/subagent";
import { AgentCard } from "./AgentCard";
import { subagentThreadToCardData } from "./AgentCard/utils";
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
                <div className="pl-4 border-l-2 border-muted">
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

interface SubagentThread {
  key: string;
  groupKey: string;
  context: SubagentContext;
  events: AnyAgentEvent[];
  subtaskIndex: number;
  firstSeenAt: number | null;
}

interface PartitionResult {
  mainStream: AnyAgentEvent[];
  subagentGroups: Map<string, SubagentThread[]>;
  pendingTools: Map<string, WorkflowToolStartedEvent>;
  resolvePairedToolStart: (event: AnyAgentEvent) => WorkflowToolStartedEvent | undefined;
}

// ============================================================================
// Event Partitioning
// ============================================================================

function partitionEvents(events: AnyAgentEvent[], isRunning: boolean): PartitionResult {
  const mainStream: AnyAgentEvent[] = [];
  const subagentThreads = new Map<string, SubagentThread>();
  const startedTools = new Map<string, WorkflowToolStartedEvent>();
  const completedToolIds = new Set<string>();

  // First pass: track lifecycle states
  events.forEach((event) => {
    if (isSubagentLike(event)) {
      return;
    }
    if (isSubagentToolEvent(event, "workflow.tool.started") || isSubagentToolEvent(event, "workflow.tool.completed")) {
      return;
    }
    const sessionId =
      typeof event.session_id === "string" ? event.session_id : "";

    if (isEventType(event, "workflow.tool.started")) {
      startedTools.set(`${sessionId}:${event.call_id}`, event);
    } else if (isEventType(event, "workflow.tool.completed")) {
      completedToolIds.add(`${sessionId}:${event.call_id}`);
    }
  });

  // Find pending (started but not completed), excluding orchestrator tools
  const pendingTools = new Map<string, WorkflowToolStartedEvent>();
  startedTools.forEach((started, key) => {
    if (!completedToolIds.has(key) && !isOrchestratorTool(started)) {
      pendingTools.set(key, started);
    }
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
      const groupKey = getSubagentGroupKey(event);
      if (!groupKey) {
        return;
      }
      const threadKey = getSubagentThreadKey(event, groupKey);
      const context = getSubagentContext(event);

      if (!subagentThreads.has(threadKey)) {
        subagentThreads.set(threadKey, {
          key: threadKey,
          groupKey,
          context,
          events: [],
          subtaskIndex: getSubtaskIndex(event),
          firstSeenAt: parseEventTimestamp(event),
        });
      }

      const thread = subagentThreads.get(threadKey)!;
      thread.context = mergeSubagentContext(thread.context, context);

      if (shouldDisplayInSubagentCard(event)) {
        thread.events.push(event);
      }
      return;
    }

    // Core agent events → main stream
    if (shouldDisplayInMainStream(event, isRunning)) {
      mainStream.push(event);
    }
  });

  const subagentGroups = new Map<string, SubagentThread[]>();
  subagentThreads.forEach((thread) => {
    const group = subagentGroups.get(thread.groupKey);
    if (!group) {
      subagentGroups.set(thread.groupKey, [thread]);
    } else {
      group.push(thread);
    }
  });

  return { mainStream, subagentGroups, pendingTools, resolvePairedToolStart };
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

  // Only show subagent tool starts (render as AgentCard); other starts
  // are handled by pendingTools while running, completed merges started params
  if (isEventType(event, "workflow.tool.started")) {
    return isSubagentToolEvent(event, "workflow.tool.started");
  }
  if (isEventType(event, "workflow.tool.completed")) {
    return !isSubagentToolCompleted(event);
  }

  // Skip internal lifecycle
  if (isEventType(event, "workflow.node.started", "workflow.node.completed")) return false;

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

function isSubagentToolStarted(event: AnyAgentEvent): event is WorkflowToolStartedEvent {
  return isSubagentToolEvent(event, "workflow.tool.started");
}

function isSubagentToolCompleted(event: AnyAgentEvent): boolean {
  return isSubagentToolEvent(event, "workflow.tool.completed");
}

function isSubagentToolEvent(
  event: AnyAgentEvent,
  kind: "workflow.tool.started" | "workflow.tool.completed",
): boolean {
  if (!isEventType(event, kind)) {
    return false;
  }
  const toolName =
    "tool_name" in event && typeof event.tool_name === "string"
      ? event.tool_name.toLowerCase()
      : "";
  return toolName === "subagent";
}

function isOrchestratorTool(event: AnyAgentEvent): boolean {
  const toolName =
    "tool_name" in event && typeof event.tool_name === "string"
      ? event.tool_name.toLowerCase()
      : "";
  return toolName === "plan" || toolName === "clarify";
}

function getSubagentGroupKey(event: AnyAgentEvent): string | null {
  // parent_run_id first: all events from the same subagent share this key
  // (= core agent's run_id), which the AgentCard render uses for lookup.
  // causation_id/call_id are per-tool-call and would fragment the group.
  const parentRunId =
    "parent_run_id" in event && typeof event.parent_run_id === "string"
      ? event.parent_run_id
      : "";
  if (parentRunId.trim()) {
    return parentRunId.trim();
  }
  const causationId =
    "causation_id" in event && typeof event.causation_id === "string"
      ? event.causation_id
      : "";
  if (causationId.trim()) {
    return causationId.trim();
  }
  const callId =
    "call_id" in event && typeof event.call_id === "string"
      ? event.call_id
      : "";
  if (callId.trim()) {
    return callId.trim();
  }
  return null;
}

function getSubagentThreadKey(event: AnyAgentEvent, groupKey: string): string {
  const runId =
    "run_id" in event && typeof event.run_id === "string" ? event.run_id : "";
  if (runId.trim()) {
    return `${groupKey}|${runId.trim()}`;
  }
  const callId =
    "call_id" in event && typeof event.call_id === "string" ? event.call_id : "";
  if (callId.trim()) {
    return `${groupKey}|call:${callId.trim()}`;
  }
  const eventId =
    "event_id" in event && typeof event.event_id === "string" ? event.event_id : "";
  if (eventId.trim()) {
    return `${groupKey}|evt:${eventId.trim()}`;
  }
  const ts = event.timestamp ?? Date.now().toString();
  return `${groupKey}|unknown:${ts}`;
}

function shouldDisplayInSubagentCard(event: AnyAgentEvent): boolean {
  return isEventType(
    event,
    "workflow.tool.started",
    "workflow.tool.completed",
    "workflow.result.final",
    "workflow.result.cancelled",
    "workflow.node.output.summary",
    "workflow.node.failed",
    "workflow.subflow.completed",
  );
}

function getSubtaskIndex(event: AnyAgentEvent): number {
  return "subtask_index" in event && typeof event.subtask_index === "number"
    ? event.subtask_index
    : Number.POSITIVE_INFINITY;
}

function mergeSubagentContext(
  existing: SubagentContext,
  incoming: SubagentContext,
): SubagentContext {
  return {
    ...existing,
    ...incoming,
    preview: incoming.preview ?? existing.preview,
    concurrency: incoming.concurrency ?? existing.concurrency,
    progress: incoming.progress ?? existing.progress,
    stats: incoming.stats ?? existing.stats,
    status: incoming.status ?? existing.status,
    statusTone: incoming.statusTone ?? existing.statusTone,
  };
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
