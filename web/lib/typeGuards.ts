// Type guard functions for discriminating agent event types
// These allow safe narrowing of union types without unsafe type assertions

import {
  AnyAgentEvent,
  WorkflowNodeStartedEvent,
  WorkflowNodeOutputDeltaEvent,
  WorkflowNodeOutputSummaryEvent,
  WorkflowToolStartedEvent,
  WorkflowToolProgressEvent,
  WorkflowToolCompletedEvent,
  WorkflowNodeCompletedEvent,
  WorkflowResultFinalEvent,
  WorkflowResultCancelledEvent,
  WorkflowNodeFailedEvent,
  WorkflowInputReceivedEvent,
  eventMatches,
} from '@/lib/types';

// Base event type guard
export function isAgentEvent(event: unknown): event is AnyAgentEvent {
  return (
    typeof event === 'object' &&
    event !== null &&
    'event_type' in event &&
    'timestamp' in event &&
    'agent_level' in event
  );
}

// Iteration Start Event (iteration-level)
export function isIterationNodeStartedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeStartedEvent & { iteration: number } {
  const nodeKind = 'node_kind' in event ? (event as any).node_kind : undefined;
  const nodeId = 'node_id' in event ? (event as any).node_id : undefined;
  const stepIndex = (event as any).step_index;
  return (
    eventMatches(event, 'workflow.node.started') &&
    typeof (event as any).iteration === 'number' &&
    (nodeKind === 'iteration' ||
      (typeof nodeId === 'string' && nodeId.startsWith('iteration-')) ||
      typeof stepIndex !== 'number')
  );
}

// Thinking Event
export function isWorkflowNodeOutputDeltaEvent(event: AnyAgentEvent): event is WorkflowNodeOutputDeltaEvent {
  return eventMatches(event, 'workflow.node.output.delta');
}

// Think Complete Event
export function isWorkflowNodeOutputSummaryEvent(event: AnyAgentEvent): event is WorkflowNodeOutputSummaryEvent {
  return eventMatches(event, 'workflow.node.output.summary');
}

// Tool Call Start Event
export function isWorkflowToolStartedEvent(event: AnyAgentEvent): event is WorkflowToolStartedEvent {
  return eventMatches(event, 'workflow.tool.started');
}

// Tool Call Stream Event
export function isWorkflowToolProgressEvent(event: AnyAgentEvent): event is WorkflowToolProgressEvent {
  return eventMatches(event, 'workflow.tool.progress');
}

// Tool Call Complete Event
export function isWorkflowToolCompletedEvent(event: AnyAgentEvent): event is WorkflowToolCompletedEvent {
  return eventMatches(event, 'workflow.tool.completed');
}

// Iteration Complete Event (iteration-level)
export function isIterationNodeCompletedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeCompletedEvent & { iteration: number } {
  const nodeKind = 'node_kind' in event ? (event as any).node_kind : undefined;
  const nodeId = 'node_id' in event ? (event as any).node_id : undefined;
  const stepIndex = (event as any).step_index;
  return (
    eventMatches(event, 'workflow.node.completed') &&
    typeof (event as any).iteration === 'number' &&
    (nodeKind === 'iteration' ||
      (typeof nodeId === 'string' && nodeId.startsWith('iteration-')) ||
      typeof stepIndex !== 'number')
  );
}

// Task Complete Event
export function isWorkflowResultFinalEvent(event: AnyAgentEvent): event is WorkflowResultFinalEvent {
  return eventMatches(event, 'workflow.result.final');
}

// Task Cancelled Event
export function isWorkflowResultCancelledEvent(event: AnyAgentEvent): event is WorkflowResultCancelledEvent {
  return eventMatches(event, 'workflow.result.cancelled');
}

// Error Event
export function isWorkflowNodeFailedEvent(event: AnyAgentEvent): event is WorkflowNodeFailedEvent {
  return eventMatches(event, 'workflow.node.failed');
}

// Step Started Event
export function isWorkflowNodeStartedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeStartedEvent & { step_index: number } {
  return eventMatches(event, 'workflow.node.started') && typeof (event as any).step_index === 'number';
}

// Step Completed Event
export function isWorkflowNodeCompletedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeCompletedEvent & { step_index: number } {
  return eventMatches(event, 'workflow.node.completed') && typeof (event as any).step_index === 'number';
}

// User Task Event
export function isWorkflowInputReceivedEvent(event: AnyAgentEvent): event is WorkflowInputReceivedEvent {
  return event.event_type === 'workflow.input.received';
}

// Composite type guards for common patterns

// Check if event is a tool-related event
export function isToolEvent(
  event: AnyAgentEvent
): event is WorkflowToolStartedEvent | WorkflowToolProgressEvent | WorkflowToolCompletedEvent {
  return (
    isWorkflowToolStartedEvent(event) ||
    isWorkflowToolProgressEvent(event) ||
    isWorkflowToolCompletedEvent(event)
  );
}

// Check if event is an iteration-related event
export function isIterationEvent(
  event: AnyAgentEvent
): event is WorkflowNodeStartedEvent | WorkflowNodeCompletedEvent {
  return isIterationNodeStartedEvent(event) || isIterationNodeCompletedEvent(event);
}

// Check if event is a step-related event
export function isStepEvent(
  event: AnyAgentEvent
): event is WorkflowNodeStartedEvent | WorkflowNodeCompletedEvent {
  return isWorkflowNodeStartedEvent(event) || isWorkflowNodeCompletedEvent(event);
}

// Check if event has iteration field
export function hasIteration(
  event: AnyAgentEvent
): event is Extract<
  AnyAgentEvent,
  { iteration: number }
> {
  return 'iteration' in event && typeof (event as any).iteration === 'number';
}

// Check if event has call_id field (tool call events)
export function hasCallId(
  event: AnyAgentEvent
): event is Extract<AnyAgentEvent, { call_id: string }> {
  return 'call_id' in event && typeof (event as any).call_id === 'string';
}

// Check if event has arguments field (tool call start)
export function hasArguments(
  event: AnyAgentEvent
): event is Extract<AnyAgentEvent, { arguments: Record<string, any> }> {
  return 'arguments' in event && typeof (event as any).arguments === 'object';
}
