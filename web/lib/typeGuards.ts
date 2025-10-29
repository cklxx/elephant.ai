// Type guard functions for discriminating agent event types
// These allow safe narrowing of union types without unsafe type assertions

import {
  AnyAgentEvent,
  TaskAnalysisEvent,
  IterationStartEvent,
  ThinkingEvent,
  ThinkCompleteEvent,
  ToolCallStartEvent,
  ToolCallStreamEvent,
  ToolCallCompleteEvent,
  IterationCompleteEvent,
  TaskCompleteEvent,
  ErrorEvent,
  ResearchPlanEvent,
  StepStartedEvent,
  StepCompletedEvent,
  BrowserInfoEvent,
  UserTaskEvent,
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

// Task Analysis Event
export function isTaskAnalysisEvent(event: AnyAgentEvent): event is TaskAnalysisEvent {
  return event.event_type === 'task_analysis';
}

// Iteration Start Event
export function isIterationStartEvent(event: AnyAgentEvent): event is IterationStartEvent {
  return event.event_type === 'iteration_start';
}

// Thinking Event
export function isThinkingEvent(event: AnyAgentEvent): event is ThinkingEvent {
  return event.event_type === 'thinking';
}

// Think Complete Event
export function isThinkCompleteEvent(event: AnyAgentEvent): event is ThinkCompleteEvent {
  return event.event_type === 'think_complete';
}

// Tool Call Start Event
export function isToolCallStartEvent(event: AnyAgentEvent): event is ToolCallStartEvent {
  return event.event_type === 'tool_call_start';
}

// Tool Call Stream Event
export function isToolCallStreamEvent(event: AnyAgentEvent): event is ToolCallStreamEvent {
  return event.event_type === 'tool_call_stream';
}

// Tool Call Complete Event
export function isToolCallCompleteEvent(event: AnyAgentEvent): event is ToolCallCompleteEvent {
  return event.event_type === 'tool_call_complete';
}

// Iteration Complete Event
export function isIterationCompleteEvent(event: AnyAgentEvent): event is IterationCompleteEvent {
  return event.event_type === 'iteration_complete';
}

// Task Complete Event
export function isTaskCompleteEvent(event: AnyAgentEvent): event is TaskCompleteEvent {
  return event.event_type === 'task_complete';
}

// Error Event
export function isErrorEvent(event: AnyAgentEvent): event is ErrorEvent {
  return event.event_type === 'error';
}

// Research Plan Event
export function isResearchPlanEvent(event: AnyAgentEvent): event is ResearchPlanEvent {
  return event.event_type === 'research_plan';
}

// Step Started Event
export function isStepStartedEvent(event: AnyAgentEvent): event is StepStartedEvent {
  return event.event_type === 'step_started';
}

// Step Completed Event
export function isStepCompletedEvent(event: AnyAgentEvent): event is StepCompletedEvent {
  return event.event_type === 'step_completed';
}

// Browser Info Event
export function isBrowserInfoEvent(event: AnyAgentEvent): event is BrowserInfoEvent {
  return event.event_type === 'browser_info';
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
