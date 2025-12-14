// Event aggregation logic for grouping and organizing agent events
// Implements research console style step tracking and tool call grouping

import {
  AnyAgentEvent,
  WorkflowToolStartedEvent,
  WorkflowToolProgressEvent,
  WorkflowToolCompletedEvent,
  WorkflowNodeStartedEvent,
  WorkflowNodeCompletedEvent,
  WorkflowNodeFailedEvent,
  AttachmentPayload,
  eventMatches,
} from './types';

/**
 * Aggregated tool call - combines start, stream chunks, and completion
 */
export interface AggregatedToolCall {
  id: string;
  call_id: string;
  tool_name: string;
  arguments: Record<string, any>;
  arguments_preview?: string;
  status: 'running' | 'streaming' | 'complete' | 'error';
  stream_chunks: string[];
  result?: string;
  error?: string;
  duration?: number;
  completed_at?: string;
  last_stream_at?: string;
  timestamp: string;
  iteration: number;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
}

/**
 * Iteration group - all events within a single ReAct iteration
 */
export interface IterationGroup {
  id: string;
  iteration: number;
  total_iters: number;
  status: 'running' | 'complete';
  started_at: string;
  completed_at?: string;
  delta?: string;
  tool_calls: AggregatedToolCall[];
  tokens_used?: number;
  tools_run?: number;
  errors: string[];
}

/**
 * Research step - high-level plan step tracking
 */
export interface ResearchStep {
  id: string;
  step_index: number;
  description: string;
  status: 'pending' | 'in_progress' | 'completed';
  started_at?: string;
  completed_at?: string;
  result?: string;
  iterations: number[];
}

/**
 * Aggregate events into tool calls by call_id
 */
export function aggregateToolCalls(events: AnyAgentEvent[]): Map<string, AggregatedToolCall> {
  const toolCallMap = new Map<string, AggregatedToolCall>();

  for (const event of events) {
    if (eventMatches(event, 'workflow.tool.started')) {
      const startEvent = event as WorkflowToolStartedEvent;
      toolCallMap.set(startEvent.call_id, {
        id: startEvent.call_id,
        call_id: startEvent.call_id,
        tool_name: startEvent.tool_name,
        arguments: startEvent.arguments,
        arguments_preview: startEvent.arguments_preview,
        status: 'running',
        stream_chunks: [],
        timestamp: startEvent.timestamp,
        iteration: startEvent.iteration ?? 0,
      });
    } else if (eventMatches(event, 'workflow.tool.progress')) {
      const streamEvent = event as WorkflowToolProgressEvent;
      const existing = toolCallMap.get(streamEvent.call_id);
      if (existing) {
        existing.status = 'streaming';
        existing.stream_chunks.push(streamEvent.chunk);
        existing.last_stream_at = streamEvent.timestamp;
      }
    } else if (eventMatches(event, 'workflow.tool.completed')) {
      const completeEvent = event as WorkflowToolCompletedEvent;
      const existing = toolCallMap.get(completeEvent.call_id);
      if (existing) {
        existing.status = completeEvent.error ? 'error' : 'complete';
        existing.result = completeEvent.result;
        existing.error = completeEvent.error;
        existing.duration = completeEvent.duration;
        existing.completed_at = completeEvent.timestamp;
        existing.metadata = completeEvent.metadata as Record<string, any> | undefined;
        existing.attachments = completeEvent.attachments as Record<string, AttachmentPayload> | undefined;
      } else {
        // Handle case where complete arrives before start
        toolCallMap.set(completeEvent.call_id, {
          id: completeEvent.call_id,
          call_id: completeEvent.call_id,
          tool_name: completeEvent.tool_name,
          arguments: {},
          arguments_preview: undefined,
          status: completeEvent.error ? 'error' : 'complete',
          stream_chunks: [],
          result: completeEvent.result,
          error: completeEvent.error,
          duration: completeEvent.duration,
          completed_at: completeEvent.timestamp,
          timestamp: completeEvent.timestamp,
          iteration: 0, // Unknown iteration
          metadata: completeEvent.metadata as Record<string, any> | undefined,
          attachments: completeEvent.attachments as Record<string, AttachmentPayload> | undefined,
        });
      }
    }
  }

  return toolCallMap;
}

export interface ToolCallSummary {
  callId: string;
  toolName: string;
  status: 'running' | 'completed' | 'error';
  startedAt: string;
  completedAt?: string;
  durationMs?: number;
  argumentsPreview?: string;
  resultPreview?: string;
  errorMessage?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
  prompt?: string;
  arguments?: Record<string, any>;
  streamChunks?: string[];
}

function truncatePreview(value: string, length: number) {
  if (value.length <= length) {
    return value;
  }
  return `${value.slice(0, length)}…`;
}

function buildArgumentsPreview(args: Record<string, any> | undefined, fallback?: string) {
  if (fallback) {
    return fallback;
  }
  if (!args) {
    return undefined;
  }
  const entries = Object.entries(args).map(([key, value]) => `${key}: ${String(value)}`);
  if (entries.length === 0) {
    return undefined;
  }
  return truncatePreview(entries.join(', '), 140);
}

function buildResultPreview(result: string | undefined) {
  if (!result) {
    return undefined;
  }
  return truncatePreview(result, 160);
}

function extractPrompt(call: AggregatedToolCall): string | undefined {
  const metadataPrompt = typeof call.metadata?.prompt === 'string' ? call.metadata.prompt : undefined;
  if (metadataPrompt && metadataPrompt.trim().length > 0) {
    return metadataPrompt;
  }

  const argumentPrompt = typeof call.arguments?.prompt === 'string' ? String(call.arguments.prompt) : undefined;
  if (argumentPrompt && argumentPrompt.trim().length > 0) {
    return argumentPrompt;
  }

  const hasAttachments = call.attachments && Object.keys(call.attachments).length > 0;
  if (hasAttachments && typeof call.result === 'string' && call.result.trim().length > 0) {
    return call.result;
  }

  return undefined;
}

export function buildToolCallSummaries(events: AnyAgentEvent[]): ToolCallSummary[] {
  const aggregated = Array.from(aggregateToolCalls(events).values());
  const sorted = aggregated.sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp));

  return sorted.map((call) => {
    const status: ToolCallSummary['status'] =
      call.status === 'error'
        ? 'error'
        : call.status === 'complete'
          ? 'completed'
          : 'running';

    return {
      callId: call.call_id,
      toolName: call.tool_name,
      status,
      startedAt: call.timestamp,
      completedAt: call.completed_at,
      durationMs: call.duration,
      argumentsPreview: buildArgumentsPreview(call.arguments, call.arguments_preview),
      resultPreview: buildResultPreview(call.result),
      errorMessage: call.error,
      metadata: call.metadata,
      attachments: call.attachments,
      prompt: extractPrompt(call),
      arguments: call.arguments,
      streamChunks: call.stream_chunks,
    };
  });
}

/**
 * Group events by iteration
 */
export function groupByIteration(events: AnyAgentEvent[]): Map<number, IterationGroup> {
  const iterationMap = new Map<number, IterationGroup>();
  const toolCallMap = aggregateToolCalls(events);

  for (const event of events) {
    const nodeKind = 'node_kind' in event ? (event as any).node_kind : undefined;
    const nodeId = 'node_id' in event ? (event as any).node_id : undefined;
    const hasIteration = typeof (event as any).iteration === 'number';
    const hasStepIndex = typeof (event as any).step_index === 'number';
    const iterationValue = hasIteration ? (event as any).iteration as number : undefined;
    const isIterationKind =
      nodeKind === 'iteration' ||
      (typeof nodeId === 'string' && nodeId.startsWith('iteration-')) ||
      (!hasStepIndex && hasIteration);

    if (
      eventMatches(event, 'workflow.node.started') &&
      isIterationKind &&
      iterationValue !== undefined
    ) {
      const startedEvent = event as WorkflowNodeStartedEvent;
      iterationMap.set(iterationValue, {
        id: `iter-${iterationValue}`,
        iteration: iterationValue,
        total_iters: startedEvent.total_iters ?? 0,
        status: 'running',
        started_at: startedEvent.timestamp,
        tool_calls: [],
        errors: [],
      });
    } else if (eventMatches(event, 'workflow.node.output.summary') && iterationValue !== undefined) {
      const group = iterationMap.get(iterationValue);
      if (group) {
        group.delta = (event as any).content ?? (event as any).delta;
      }
    } else if (
      eventMatches(event, 'workflow.node.completed') &&
      isIterationKind &&
      iterationValue !== undefined
    ) {
      const completedEvent = event as WorkflowNodeCompletedEvent;
      const group = iterationMap.get(iterationValue);
      if (group) {
        group.status = 'complete';
        group.completed_at = completedEvent.timestamp;
        group.tokens_used = completedEvent.tokens_used;
        group.tools_run = completedEvent.tools_run;
      }
    } else if (
      eventMatches(event, 'workflow.node.failed') &&
      isIterationKind &&
      iterationValue !== undefined
    ) {
      const failedEvent = event as WorkflowNodeFailedEvent;
      const group = iterationMap.get(iterationValue);
      if (group) {
        const errorMessage = failedEvent.error;
        if (typeof errorMessage === 'string' && errorMessage.length > 0) {
          group.errors.push(errorMessage);
        }
      }
    }
  }

  // Assign aggregated tool calls to their iterations
  for (const toolCall of toolCallMap.values()) {
    const group = iterationMap.get(toolCall.iteration);
    if (group) {
      group.tool_calls.push(toolCall);
    }
  }

  return iterationMap;
}

/**
 * Extract research steps from events
 */
export function extractResearchSteps(events: AnyAgentEvent[]): ResearchStep[] {
  const plannedDescriptions = new Map<number, string>();
  const steps = new Map<number, ResearchStep>();

  const ensureStep = (stepIndex: number, fallbackDescription?: string): ResearchStep => {
    if (!steps.has(stepIndex)) {
      const description =
        fallbackDescription ?? plannedDescriptions.get(stepIndex) ?? `Step ${stepIndex + 1}`;

      steps.set(stepIndex, {
        id: `step-${stepIndex}`,
        step_index: stepIndex,
        description,
        status: 'pending',
        iterations: [],
      });
    }

    const step = steps.get(stepIndex)!;

    if (!step.description && fallbackDescription) {
      step.description = fallbackDescription;
    }

    return step;
  };

  for (const event of events) {
    switch (true) {
      case eventMatches(event, 'workflow.node.started'): {
        if (typeof (event as any).step_index !== 'number') {
          break;
        }
        const stepEvent = event as WorkflowNodeStartedEvent & { step_index: number };
        const step = ensureStep(stepEvent.step_index, stepEvent.step_description);
        step.status = 'in_progress';
        step.started_at = stepEvent.timestamp;
        if (typeof stepEvent.iteration === 'number' && !step.iterations.includes(stepEvent.iteration)) {
          step.iterations.push(stepEvent.iteration);
        }
        break;
      }
      case eventMatches(event, 'workflow.node.completed'): {
        if (typeof (event as any).step_index !== 'number') {
          break;
        }
        const stepEvent = event as WorkflowNodeCompletedEvent & { step_index: number };
        const step = ensureStep(stepEvent.step_index, stepEvent.step_description);
        step.status = 'completed';
        step.completed_at = stepEvent.timestamp;
        step.result = stepEvent.step_result;
        if (!step.started_at) {
          step.started_at = stepEvent.timestamp;
        }
        if (typeof stepEvent.iteration === 'number' && !step.iterations.includes(stepEvent.iteration)) {
          step.iterations.push(stepEvent.iteration);
        }
        break;
      }
      default:
        break;
    }
  }

  for (const [index, step] of steps.entries()) {
    if (!step.description) {
      step.description = plannedDescriptions.get(index) ?? `Step ${index + 1}`;
    }
  }

  return Array.from(steps.values()).sort((a, b) => a.step_index - b.step_index);
}

/**
 * LRU cache for events with configurable size
 */
export class EventLRUCache {
  private maxSize: number;
  private events: AnyAgentEvent[] = [];

  constructor(maxSize: number = 1000) {
    this.maxSize = maxSize;
  }

  add(event: AnyAgentEvent): void {
    this.events.push(event);

    // Evict oldest events if over limit
    if (this.events.length > this.maxSize) {
      const evictCount = this.events.length - this.maxSize;
      this.events.splice(0, evictCount);
    }
  }

  /**
   * Replace the most recent event if it matches the predicate. This keeps streaming
   * updates adjacent (e.g., workflow.result.final deltas) while preserving older, non-adjacent
   * events.
   *
   * @returns true if a replacement occurred, false otherwise
   */
  replaceLastIf(predicate: (event: AnyAgentEvent) => boolean, replacement: AnyAgentEvent): boolean {
    const lastIndex = this.events.length - 1;
    if (lastIndex >= 0 && predicate(this.events[lastIndex])) {
      this.events[lastIndex] = replacement;
      return true;
    }

    return false;
  }

  peekLast(): AnyAgentEvent | undefined {
    if (this.events.length === 0) return undefined;
    return this.events[this.events.length - 1];
  }

  addMany(events: AnyAgentEvent[]): void {
    events.forEach((e) => this.add(e));
  }

  getAll(): AnyAgentEvent[] {
    return this.events.slice();
  }

  clear(): void {
    this.events = [];
  }

  size(): number {
    return this.events.length;
  }

  getMemoryUsage(): { eventCount: number; estimatedBytes: number } {
    // Rough estimation: ~500 bytes per event on average
    const estimatedBytes = this.events.length * 500;
    return {
      eventCount: this.events.length,
      estimatedBytes,
    };
  }
}
