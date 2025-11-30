// Type guard functions for discriminating agent event types
// These allow safe narrowing of union types without unsafe type assertions

import {
  AnyAgentEvent,
  IterationStartEvent,
  ThinkingEvent,
  ThinkCompleteEvent,
  ToolCallStartEvent,
  ToolCallStreamEvent,
  ToolCallCompleteEvent,
  IterationCompleteEvent,
  TaskCompleteEvent,
  TaskCancelledEvent,
  ErrorEvent,
  ResearchPlanEvent,
  StepStartedEvent,
  StepCompletedEvent,
  BrowserInfoEvent,
  UserTaskEvent,
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

// Iteration Start Event
export function isIterationStartEvent(event: AnyAgentEvent): event is IterationStartEvent {
  return (
    eventMatches(event, 'workflow.node.started', 'iteration_start') &&
    (event.node_kind === 'iteration' ||
      (event as any).legacy_type === 'iteration_start' ||
      (event.node_id ?? '').startsWith('iteration-'))
  );
}

// Thinking Event
export function isThinkingEvent(event: AnyAgentEvent): event is ThinkingEvent {
  return eventMatches(event, 'workflow.node.output.delta', 'thinking');
}

// Think Complete Event
export function isThinkCompleteEvent(event: AnyAgentEvent): event is ThinkCompleteEvent {
  return eventMatches(event, 'workflow.node.output.summary', 'think_complete');
}

// Tool Call Start Event
export function isToolCallStartEvent(event: AnyAgentEvent): event is ToolCallStartEvent {
  return eventMatches(event, 'workflow.tool.started', 'tool_call_start');
}

// Tool Call Stream Event
export function isToolCallStreamEvent(event: AnyAgentEvent): event is ToolCallStreamEvent {
  return eventMatches(event, 'workflow.tool.progress', 'tool_call_stream');
}

// Tool Call Complete Event
export function isToolCallCompleteEvent(event: AnyAgentEvent): event is ToolCallCompleteEvent {
  return eventMatches(event, 'workflow.tool.completed', 'tool_call_complete');
}

// Iteration Complete Event
export function isIterationCompleteEvent(event: AnyAgentEvent): event is IterationCompleteEvent {
  return (
    eventMatches(event, 'workflow.node.completed', 'iteration_complete') &&
    (event.node_kind === 'iteration' ||
      (event as any).legacy_type === 'iteration_complete' ||
      (event.node_id ?? '').startsWith('iteration-'))
  );
}

// Task Complete Event
export function isTaskCompleteEvent(event: AnyAgentEvent): event is TaskCompleteEvent {
  return eventMatches(event, 'workflow.result.final', 'task_complete');
}

// Task Cancelled Event
export function isTaskCancelledEvent(event: AnyAgentEvent): event is TaskCancelledEvent {
  return eventMatches(event, 'workflow.result.cancelled', 'task_cancelled');
}

// Error Event
export function isErrorEvent(event: AnyAgentEvent): event is ErrorEvent {
  return eventMatches(event, 'workflow.node.failed', 'error');
}

// Research Plan Event
export function isResearchPlanEvent(event: AnyAgentEvent): event is ResearchPlanEvent {
  return eventMatches(event, 'workflow.plan.generated', 'research_plan');
}

// Step Started Event
export function isStepStartedEvent(event: AnyAgentEvent): event is StepStartedEvent {
  return eventMatches(event, 'workflow.node.started', 'step_started') && typeof (event as any).step_index === 'number';
}

// Step Completed Event
export function isStepCompletedEvent(event: AnyAgentEvent): event is StepCompletedEvent {
  return eventMatches(event, 'workflow.node.completed', 'step_completed') && typeof (event as any).step_index === 'number';
}

// Browser Info Event
export function isBrowserInfoEvent(event: AnyAgentEvent): event is BrowserInfoEvent {
  return eventMatches(event, 'workflow.diagnostic.browser_info', 'browser_info');
}

// User Task Event
export function isUserTaskEvent(event: AnyAgentEvent): event is UserTaskEvent {
  return event.event_type === 'user_task';
}

// Composite type guards for common patterns

// Check if event is a tool-related event
export function isToolEvent(
  event: AnyAgentEvent
): event is ToolCallStartEvent | ToolCallStreamEvent | ToolCallCompleteEvent {
  return (
    isToolCallStartEvent(event) ||
    isToolCallStreamEvent(event) ||
    isToolCallCompleteEvent(event)
  );
}

// Check if event is an iteration-related event
export function isIterationEvent(
  event: AnyAgentEvent
): event is IterationStartEvent | IterationCompleteEvent {
  return isIterationStartEvent(event) || isIterationCompleteEvent(event);
}

// Check if event is a step-related event
export function isStepEvent(
  event: AnyAgentEvent
): event is StepStartedEvent | StepCompletedEvent {
  return isStepStartedEvent(event) || isStepCompletedEvent(event);
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
): event is Extract<
  AnyAgentEvent,
  { call_id: string }
> {
  return 'call_id' in event && typeof (event as any).call_id === 'string';
}

// Check if event has arguments field (tool call start)
export function hasArguments(
  event: AnyAgentEvent
): event is Extract<
  AnyAgentEvent,
  { arguments: Record<string, any> }
> {
  return 'arguments' in event && typeof (event as any).arguments === 'object';
}
