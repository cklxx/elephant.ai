/**
 * Utility functions extracted from ConversationEventStream for reuse and testability.
 */

import type {
  AnyAgentEvent,
  WorkflowToolStartedEvent,
} from "@/lib/types";
import { isEventType } from "@/lib/events/matching";
import { isSubagentLike } from "@/lib/subagent";
import type { SubagentContext } from "./EventLine";
import { getSubagentContext } from "./EventLine";

// ============================================================================
// Types
// ============================================================================

export interface SubagentThread {
  key: string;
  groupKey: string;
  context: SubagentContext;
  events: AnyAgentEvent[];
  subtaskIndex: number;
  firstSeenAt: number | null;
}

export interface PartitionResult {
  mainStream: AnyAgentEvent[];
  subagentGroups: Map<string, SubagentThread[]>;
  pendingTools: Map<string, WorkflowToolStartedEvent>;
  resolvePairedToolStart: (event: AnyAgentEvent) => WorkflowToolStartedEvent | undefined;
}

// ============================================================================
// Sorting
// ============================================================================

export function parseEventTimestamp(event: AnyAgentEvent): number | null {
  const ts = Date.parse(event.timestamp ?? "");
  return Number.isFinite(ts) ? ts : null;
}

export function sortEventsBySeq(events: AnyAgentEvent[]): AnyAgentEvent[] {
  const normalized = events.map((event, index) => ({
    event,
    index,
    // seq=0 means "unassigned" (ReactEngine's SeqCounter starts at 1)
    seq: "seq" in event && typeof event.seq === "number" && event.seq > 0 ? event.seq : null,
    ts: parseEventTimestamp(event) ?? Number.MAX_SAFE_INTEGER,
  }));

  normalized.sort((a, b) => {
    // Both have valid seq → use seq as primary sort key
    if (a.seq !== null && b.seq !== null) {
      if (a.seq !== b.seq) {
        return a.seq - b.seq;
      }
      return a.index - b.index;
    }
    // At least one has no seq → fall back to timestamp so that
    // multi-turn input events are interleaved correctly
    if (a.ts !== b.ts) {
      return a.ts - b.ts;
    }
    // Same timestamp: event without seq (e.g. input) precedes events
    // with seq (the reactions it triggers)
    if (a.seq !== null || b.seq !== null) {
      return a.seq !== null ? 1 : -1;
    }
    return a.index - b.index;
  });

  return normalized.map((item) => item.event);
}

// ============================================================================
// Event Classification Helpers
// ============================================================================

export function isSubagentToolEvent(
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

export function isSubagentToolStarted(event: AnyAgentEvent): event is WorkflowToolStartedEvent {
  return isSubagentToolEvent(event, "workflow.tool.started");
}

function isSubagentToolCompleted(event: AnyAgentEvent): boolean {
  return isSubagentToolEvent(event, "workflow.tool.completed");
}

function isOrchestratorTool(event: AnyAgentEvent): boolean {
  const toolName =
    "tool_name" in event && typeof event.tool_name === "string"
      ? event.tool_name.toLowerCase()
      : "";
  return toolName === "plan" || toolName === "clarify";
}

// ============================================================================
// Display Rules
// ============================================================================

function shouldDisplayInMainStream(event: AnyAgentEvent, isRunning: boolean): boolean {
  if (isEventType(event, "workflow.input.received")) return true;

  if (isEventType(event, "workflow.node.output.delta")) {
    return isRunning && event.agent_level !== "subagent";
  }

  if (isEventType(event, "workflow.node.output.summary")) return true;

  if (isEventType(event, "workflow.result.final")) {
    // await_user_input is just a turn boundary marker, not a real completion
    const stopReason =
      "stop_reason" in event && typeof event.stop_reason === "string"
        ? event.stop_reason
        : "";
    if (stopReason === "await_user_input") {
      return false;
    }
    return true;
  }
  if (isEventType(event, "workflow.result.cancelled")) return true;

  if (isEventType(event, "workflow.tool.started")) {
    return isSubagentToolEvent(event, "workflow.tool.started");
  }
  if (isEventType(event, "workflow.tool.completed")) {
    return !isSubagentToolCompleted(event);
  }

  if (isEventType(event, "workflow.node.started", "workflow.node.completed")) return false;

  if (event.event_type.startsWith("workflow.diagnostic")) return false;

  return true;
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

// ============================================================================
// Subagent Grouping Helpers
// ============================================================================

function getSubagentGroupKey(event: AnyAgentEvent): string | null {
  const parentRunId =
    "parent_run_id" in event && typeof event.parent_run_id === "string"
      ? event.parent_run_id
      : "";
  if (parentRunId.trim()) {
    return parentRunId.trim();
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

// ============================================================================
// Event Partitioning
// ============================================================================

export function partitionEvents(events: AnyAgentEvent[], isRunning: boolean): PartitionResult {
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
// Keys
// ============================================================================

export function getEventKey(event: AnyAgentEvent, index: number): string {
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
