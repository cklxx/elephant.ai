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
  const {
    mainStream,
    subagentGroups,
    pendingTools,
    pendingNodes,
    resolvePairedToolStart,
  } = useMemo(() => partitionEvents(events, isRunning), [events, isRunning]);

  // Build unified timeline with subagent groups attached to triggers
  const timeline = useMemo(
    () => buildUnifiedTimeline(mainStream, subagentGroups),
    [mainStream, subagentGroups]
  );

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

        {/* Unified timeline */}
        {timeline.map((entry, index) => (
          <TimelineEntry
            key={getEntryKey(entry, index)}
            entry={entry}
            resolvePairedToolStart={resolvePairedToolStart}
          />
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

        {Array.from(pendingNodes.values()).map((node) => (
          <div
            key={`pending-node-${(node as any).node_id || node.timestamp}`}
            className={cn(
              "py-1 pl-2 border-primary/10",
              "group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2"
            )}
            data-testid="pending-node"
          >
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <LoadingDots />
              <span>Running: {(node as any).node_id || "node"}</span>
            </div>
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
// Timeline Entry Component
// ============================================================================

function TimelineEntry({
  entry,
  resolvePairedToolStart,
}: {
  entry: TimelineEntry;
  resolvePairedToolStart: (event: AnyAgentEvent) => WorkflowToolStartedEvent | undefined;
}) {
  switch (entry.kind) {
    case "subagentGroup": {
      if (!entry.trigger) {
        // Fallback: render only subagent cards without trigger
        return (
          <div
            className="-mx-2 px-2 my-2 flex flex-col gap-3"
            data-testid="subagent-thread-group"
          >
            <div className="pl-4 border-l-2 border-muted">
              {entry.threads.map((thread) => (
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
          className="-mx-2 px-2 my-2 flex flex-col gap-3"
          data-testid="subagent-thread-group"
        >
          {/* Trigger tool call */}
          <div className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2">
            <EventLine
              event={entry.trigger}
              pairedToolStartEvent={resolvePairedToolStart(entry.trigger)}
            />
          </div>

          {/* Subagent cards */}
          <div className="pl-4 border-l-2 border-muted">
            {entry.threads.map((thread) => (
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

    case "event":
    default:
      return (
        <div className="group transition-colors rounded-lg hover:bg-muted/10 -mx-2 px-2">
          <EventLine
            event={entry.event}
            pairedToolStartEvent={resolvePairedToolStart(entry.event)}
          />
        </div>
      );
  }
}

// ============================================================================
// Timeline Builder
// ============================================================================

function buildUnifiedTimeline(
  mainStream: AnyAgentEvent[],
  subagentGroups: Map<string, SubagentThread[]>,
): TimelineEntry[] {
  const result: TimelineEntry[] = [];
  const assignedGroups = new Set<string>();

  for (const event of mainStream) {
    // Check if this is a subagent trigger
    if (isSubagentTrigger(event)) {
      const triggerCallId =
        "call_id" in event && typeof event.call_id === "string"
          ? event.call_id
          : undefined;
      const parentRunId = event.run_id;
      const groupKey = triggerCallId || parentRunId;
      if (!groupKey) continue;

      const group = subagentGroups.get(groupKey);
      if (group && !assignedGroups.has(groupKey)) {
        // Create subagent group entry
        result.push({
          kind: "subagentGroup",
          trigger: event,
          threads: group,
          ts: Date.parse(event.timestamp ?? "") || 0,
        });
        assignedGroups.add(groupKey);
        continue;
      }
    }

    // Regular event
    result.push({
      kind: "event",
      event,
      ts: Date.parse(event.timestamp ?? "") || 0,
    });
  }

  // Add any unassigned groups at the end (fallback)
  subagentGroups.forEach((threads, groupKey) => {
    if (!assignedGroups.has(groupKey)) {
      const earliestTs = threads.reduce((min, t) => {
        return t.firstSeenAt !== null && t.firstSeenAt < min ? t.firstSeenAt : min;
      }, Number.POSITIVE_INFINITY);

      result.push({
        kind: "subagentGroup",
        trigger: null,
        threads,
        ts: earliestTs === Number.POSITIVE_INFINITY ? Date.now() : earliestTs,
      });
    }
  });

  return result;
}

function isSubagentTrigger(event: AnyAgentEvent): boolean {
  return (
    isEventType(event, "workflow.tool.completed") &&
    "tool_name" in event &&
    typeof event.tool_name === "string" &&
    event.tool_name.toLowerCase() === "subagent"
  );
}

function getEntryKey(entry: TimelineEntry, index: number): string {
  if (entry.kind === "subagentGroup") {
    if (entry.trigger) {
      const id = (entry.trigger as any).call_id || entry.trigger.run_id || index;
      return `subagent-group-${id}`;
    }
    return `subagent-group-unassigned-${index}`;
  }
  return `event-${entry.event.event_type}-${index}`;
}

// ============================================================================
// Types
// ============================================================================

interface SubagentThread {
  key: string;
  context: SubagentContext;
  events: AnyAgentEvent[];
  subtaskIndex: number;
  firstSeenAt: number | null;
}

type TimelineEntry =
  | { kind: "event"; event: AnyAgentEvent; ts: number }
  | { kind: "subagentGroup"; trigger: AnyAgentEvent | null; threads: SubagentThread[]; ts: number };

interface PartitionResult {
  mainStream: AnyAgentEvent[];
  subagentGroups: Map<string, SubagentThread[]>;
  pendingTools: Map<string, WorkflowToolStartedEvent>;
  pendingNodes: Map<string, AnyAgentEvent>;
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
  const startedNodes = new Map<string, AnyAgentEvent>();
  const completedNodeIds = new Set<string>();

  // First pass: track lifecycle states
  events.forEach((event) => {
    const sessionId = typeof event.session_id === "string" ? event.session_id : "";

    if (isEventType(event, "workflow.tool.started")) {
      startedTools.set(`${sessionId}:${event.call_id}`, event);
    } else if (isEventType(event, "workflow.tool.completed")) {
      completedToolIds.add(`${sessionId}:${event.call_id}`);
    } else if (isEventType(event, "workflow.node.started")) {
      const nodeId = (event as any).node_id || "";
      startedNodes.set(`${sessionId}:${nodeId}`, event);
    } else if (isEventType(event, "workflow.node.completed")) {
      const nodeId = (event as any).node_id || "";
      completedNodeIds.add(`${sessionId}:${nodeId}`);
    }
  });

  // Find pending (started but not completed)
  const pendingTools = new Map<string, WorkflowToolStartedEvent>();
  startedTools.forEach((started, key) => {
    if (!completedToolIds.has(key)) pendingTools.set(key, started);
  });

  const pendingNodes = new Map<string, AnyAgentEvent>();
  startedNodes.forEach((started, key) => {
    if (!completedNodeIds.has(key)) pendingNodes.set(key, started);
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
    // Subagent events → aggregate into threads
    if (isSubagentLike(event)) {
      const key = getSubagentKey(event);
      const context = getSubagentContext(event);

      if (!subagentThreads.has(key)) {
        subagentThreads.set(key, {
          key,
          context,
          events: [],
          subtaskIndex: getSubtaskIndex(event),
          firstSeenAt: parseEventTimestamp(event),
        });
      }

      const thread = subagentThreads.get(key)!;
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

  // Group subagent threads by causation_id or parent_run_id (the key prefix)
  const subagentGroups = new Map<string, SubagentThread[]>();
  subagentThreads.forEach((thread) => {
    const groupKey = thread.key.split(":")[0];
    if (!groupKey) return;

    const group = subagentGroups.get(groupKey);
    if (!group) {
      subagentGroups.set(groupKey, [thread]);
    } else {
      group.push(thread);
    }
  });

  return { mainStream, subagentGroups, pendingTools, pendingNodes, resolvePairedToolStart };
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

function shouldDisplayInSubagentCard(event: AnyAgentEvent): boolean {
  // Only show meaningful completion events in subagent cards
  return isEventType(
    event,
    "workflow.tool.completed",
    "workflow.result.final",
    "workflow.result.cancelled",
    "workflow.node.output.summary",
    "workflow.node.failed",
  );
}

// ============================================================================
// Utilities
// ============================================================================

function parseEventTimestamp(event: AnyAgentEvent): number | null {
  const ts = Date.parse(event.timestamp ?? "");
  return Number.isFinite(ts) ? ts : null;
}

function getSubagentKey(event: AnyAgentEvent): string {
  const causationId =
    "causation_id" in event && typeof event.causation_id === "string"
      ? event.causation_id
      : undefined;
  const parentRunId =
    "parent_run_id" in event && typeof event.parent_run_id === "string"
      ? event.parent_run_id
      : undefined;
  const runId =
    "run_id" in event && typeof event.run_id === "string" ? event.run_id : undefined;
  const callId =
    "call_id" in event && typeof event.call_id === "string" ? event.call_id : undefined;

  const groupPrefix = causationId || parentRunId;
  if (groupPrefix) {
    if (runId) return `${groupPrefix}:${runId}`;
    if (callId) return `${groupPrefix}:call:${callId}`;
    return `parent:${groupPrefix}`;
  }
  if (runId) return `run:${runId}`;
  if (callId) return `call:${callId}`;
  return `unknown:${event.timestamp || Date.now()}`;
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
