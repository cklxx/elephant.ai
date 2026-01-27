import { AnyAgentEvent } from "@/lib/types";
import { SubagentContext } from "../EventLine";
import { AgentCardData, AgentCardState } from "./types";

export function subagentThreadToCardData(
  key: string,
  context: SubagentContext,
  events: AnyAgentEvent[],
  subtaskIndex: number,
): AgentCardData {
  const state = inferState(events, context);
  const progress = parseProgress(context.progress);
  const stats = parseStats(context.stats);
  const concurrency = parseConcurrency(context.concurrency, subtaskIndex);

  return {
    id: key,
    state,
    preview: context.preview,
    progress,
    stats,
    concurrency,
    events,
    statusTone: context.statusTone,
  };
}

function inferState(
  events: AnyAgentEvent[],
  context: SubagentContext,
): AgentCardState {
  if (events.length === 0) {
    return "idle";
  }

  const lastEvent = events[events.length - 1];

  if (lastEvent.event_type === "workflow.result.final") {
    return "completed";
  }

  if (
    lastEvent.event_type === "workflow.node.failed" ||
    context.statusTone === "danger"
  ) {
    return "failed";
  }

  if (lastEvent.event_type === "workflow.result.cancelled") {
    return "cancelled";
  }

  if (
    lastEvent.event_type === "workflow.subflow.completed" &&
    context.statusTone === "success"
  ) {
    return "completed";
  }

  return "running";
}

function parseProgress(
  progressStr?: string,
): { current: number; total: number; percentage: number } | undefined {
  if (!progressStr) {
    return undefined;
  }

  const match = progressStr.match(/Progress (\d+)\/(\d+)/);
  if (!match) {
    return undefined;
  }

  const current = parseInt(match[1], 10);
  const total = parseInt(match[2], 10);

  if (isNaN(current) || isNaN(total) || total === 0) {
    return undefined;
  }

  return {
    current,
    total,
    percentage: (current / total) * 100,
  };
}

function parseStats(statsStr?: string): {
  toolCalls: number;
  tokens: number;
  duration?: number;
} {
  const result = {
    toolCalls: 0,
    tokens: 0,
    duration: undefined as number | undefined,
  };

  if (!statsStr) {
    return result;
  }

  const toolCallsMatch = statsStr.match(/(\d+)\s+tool\s+calls?/);
  if (toolCallsMatch) {
    result.toolCalls = parseInt(toolCallsMatch[1], 10);
  }

  const tokensMatch = statsStr.match(/(\d+)\s+tokens/);
  if (tokensMatch) {
    result.tokens = parseInt(tokensMatch[1], 10);
  }

  return result;
}

function parseConcurrency(
  concurrencyStr?: string,
  subtaskIndex?: number,
): { index: number; total: number } | undefined {
  if (!concurrencyStr) {
    return undefined;
  }

  const match = concurrencyStr.match(/Parallel ×(\d+)/);
  if (!match) {
    return undefined;
  }

  const total = parseInt(match[1], 10);
  if (isNaN(total) || total <= 1) {
    return undefined;
  }

  return {
    index: subtaskIndex !== undefined ? subtaskIndex + 1 : 1,
    total,
  };
}
