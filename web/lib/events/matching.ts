import type { AnyAgentEvent, AgentEventType } from "@/lib/types";

export function isEventType<T extends AgentEventType>(
  event: AnyAgentEvent,
  ...types: T[]
): event is Extract<AnyAgentEvent, { event_type: T }> {
  return types.includes(event.event_type as T);
}

export const isTerminalEvent = (event: AnyAgentEvent): boolean =>
  isEventType(event, "workflow.result.final", "workflow.result.cancelled");

export const isStreamingEvent = (event: AnyAgentEvent): boolean =>
  isEventType(event, "workflow.node.output.delta");
