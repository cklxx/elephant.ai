// Type guard functions for discriminating agent event types
// These allow safe narrowing of union types without unsafe type assertions

import {
  AnyAgentEvent,
  WorkflowNodeStartedEvent,
  WorkflowNodeOutputDeltaEvent,
  WorkflowNodeOutputSummaryEvent,
  WorkflowToolStartedEvent,
  WorkflowToolCompletedEvent,
  WorkflowNodeCompletedEvent,
  WorkflowResultFinalEvent,
  WorkflowNodeFailedEvent,
} from '@/lib/types';
import { isEventType } from '@/lib/events/matching';

/** Safe accessor for optional properties on narrowed event unions */
function prop<T>(event: AnyAgentEvent, key: string): T | undefined {
  return key in event ? (event as unknown as Record<string, unknown>)[key] as T : undefined;
}

// Iteration Start Event (iteration-level)
export function isIterationNodeStartedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeStartedEvent & { iteration: number } {
  const nodeKind = prop<string>(event, 'node_kind');
  const nodeId = prop<string>(event, 'node_id');
  const stepIndex = prop<number>(event, 'step_index');
  return (
    isEventType(event, 'workflow.node.started') &&
    typeof prop<number>(event, 'iteration') === 'number' &&
    (nodeKind === 'iteration' ||
      (typeof nodeId === 'string' && nodeId.startsWith('iteration-')) ||
      typeof stepIndex !== 'number')
  );
}

// Thinking Event
export function isWorkflowNodeOutputDeltaEvent(event: AnyAgentEvent): event is WorkflowNodeOutputDeltaEvent {
  return isEventType(event, 'workflow.node.output.delta');
}

// Think Complete Event
export function isWorkflowNodeOutputSummaryEvent(event: AnyAgentEvent): event is WorkflowNodeOutputSummaryEvent {
  return isEventType(event, 'workflow.node.output.summary');
}

// Tool Call Start Event
export function isWorkflowToolStartedEvent(event: AnyAgentEvent): event is WorkflowToolStartedEvent {
  return isEventType(event, 'workflow.tool.started');
}

// Tool Call Complete Event
export function isWorkflowToolCompletedEvent(event: AnyAgentEvent): event is WorkflowToolCompletedEvent {
  return isEventType(event, 'workflow.tool.completed');
}

// Iteration Complete Event (iteration-level)
export function isIterationNodeCompletedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeCompletedEvent & { iteration: number } {
  const nodeKind = prop<string>(event, 'node_kind');
  const nodeId = prop<string>(event, 'node_id');
  const stepIndex = prop<number>(event, 'step_index');
  return (
    isEventType(event, 'workflow.node.completed') &&
    typeof prop<number>(event, 'iteration') === 'number' &&
    (nodeKind === 'iteration' ||
      (typeof nodeId === 'string' && nodeId.startsWith('iteration-')) ||
      typeof stepIndex !== 'number')
  );
}

// Task Complete Event
export function isWorkflowResultFinalEvent(event: AnyAgentEvent): event is WorkflowResultFinalEvent {
  return isEventType(event, 'workflow.result.final');
}

// Error Event
export function isWorkflowNodeFailedEvent(event: AnyAgentEvent): event is WorkflowNodeFailedEvent {
  return isEventType(event, 'workflow.node.failed');
}

// Step Started Event
export function isWorkflowNodeStartedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeStartedEvent & { step_index: number } {
  return isEventType(event, 'workflow.node.started') && typeof prop<number>(event, 'step_index') === 'number';
}

// Step Completed Event
export function isWorkflowNodeCompletedEvent(
  event: AnyAgentEvent,
): event is WorkflowNodeCompletedEvent & { step_index: number } {
  return isEventType(event, 'workflow.node.completed') && typeof prop<number>(event, 'step_index') === 'number';
}

// Utility guards

// Check if event has iteration field
export function hasIteration(
  event: AnyAgentEvent
): event is Extract<
  AnyAgentEvent,
  { iteration: number }
> {
  return 'iteration' in event && typeof prop<number>(event, 'iteration') === 'number';
}
