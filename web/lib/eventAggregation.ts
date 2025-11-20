// Event aggregation logic for grouping and organizing agent events
// Implements research console style step tracking and tool call grouping

import {
  AnyAgentEvent,
  ToolCallStartEvent,
  ToolCallStreamEvent,
  ToolCallCompleteEvent,
  AttachmentPayload,
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
  thinking?: string;
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
 * Browser diagnostics data
 */
export interface BrowserDiagnostics {
  id: string;
  timestamp: string;
  captured: string;
  success?: boolean;
  message?: string;
  user_agent?: string;
  cdp_url?: string;
  vnc_url?: string;
  viewport_width?: number;
  viewport_height?: number;
}

/**
 * Aggregate events into tool calls by call_id
 */
export function aggregateToolCalls(events: AnyAgentEvent[]): Map<string, AggregatedToolCall> {
  const toolCallMap = new Map<string, AggregatedToolCall>();

  for (const event of events) {
    if (event.event_type === 'tool_call_start') {
      const startEvent = event as ToolCallStartEvent;
      toolCallMap.set(startEvent.call_id, {
        id: startEvent.call_id,
        call_id: startEvent.call_id,
        tool_name: startEvent.tool_name,
        arguments: startEvent.arguments,
        arguments_preview: startEvent.arguments_preview,
        status: 'running',
        stream_chunks: [],
        timestamp: startEvent.timestamp,
        iteration: startEvent.iteration,
      });
    } else if (event.event_type === 'tool_call_stream') {
      const streamEvent = event as ToolCallStreamEvent;
      const existing = toolCallMap.get(streamEvent.call_id);
      if (existing) {
        existing.status = 'streaming';
        existing.stream_chunks.push(streamEvent.chunk);
        existing.last_stream_at = streamEvent.timestamp;
      }
    } else if (event.event_type === 'tool_call_complete') {
      const completeEvent = event as ToolCallCompleteEvent;
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

export type SandboxLevel = 'standard' | 'filesystem' | 'system';

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
  requiresSandbox: boolean;
  sandboxLevel: SandboxLevel;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
  prompt?: string;
  arguments?: Record<string, any>;
  streamChunks?: string[];
}

export const FILE_TOOL_HINTS = ['file', 'fs', 'write', 'read', 'download', 'upload'];
export const SYSTEM_TOOL_HINTS = ['shell', 'bash', 'system', 'process', 'exec', 'command'];

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

function inferSandboxLevel(toolName: string): SandboxLevel {
  const normalized = toolName.toLowerCase();

  if (SYSTEM_TOOL_HINTS.some((hint) => normalized.includes(hint))) {
    return 'system';
  }

  if (FILE_TOOL_HINTS.some((hint) => normalized.includes(hint))) {
    return 'filesystem';
  }

  return 'standard';
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

    const sandboxLevel = inferSandboxLevel(call.tool_name);

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
      requiresSandbox: true,
      sandboxLevel,
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
    if (event.event_type === 'iteration_start') {
      iterationMap.set(event.iteration, {
        id: `iter-${event.iteration}`,
        iteration: event.iteration,
        total_iters: event.total_iters,
        status: 'running',
        started_at: event.timestamp,
        tool_calls: [],
        errors: [],
      });
    } else if (event.event_type === 'think_complete') {
      const group = iterationMap.get(event.iteration);
      if (group) {
        group.thinking = event.content;
      }
    } else if (event.event_type === 'iteration_complete') {
      const group = iterationMap.get(event.iteration);
      if (group) {
        group.status = 'complete';
        group.completed_at = event.timestamp;
        group.tokens_used = event.tokens_used;
        group.tools_run = event.tools_run;
      }
    } else if (event.event_type === 'error') {
      const group = iterationMap.get(event.iteration);
      if (group) {
        group.errors.push(event.error);
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
    switch (event.event_type) {
      case 'research_plan':
        event.plan_steps.forEach((description, index) => {
          plannedDescriptions.set(index, description);
          ensureStep(index, description);
        });
        break;
      case 'step_started': {
        const step = ensureStep(event.step_index, event.step_description);
        step.status = 'in_progress';
        step.started_at = event.timestamp;
        if (typeof event.iteration === 'number' && !step.iterations.includes(event.iteration)) {
          step.iterations.push(event.iteration);
        }
        break;
      }
      case 'step_completed': {
        const step = ensureStep(event.step_index, event.step_description);
        step.status = 'completed';
        step.completed_at = event.timestamp;
        step.result = event.step_result;
        if (!step.started_at) {
          step.started_at = event.timestamp;
        }
        if (typeof event.iteration === 'number' && !step.iterations.includes(event.iteration)) {
          step.iterations.push(event.iteration);
        }
        break;
      }
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
 * Extract browser diagnostics events from the stream
 */
export function extractBrowserDiagnostics(events: AnyAgentEvent[]): BrowserDiagnostics[] {
  return events
    .filter((e): e is import('./types').BrowserInfoEvent => e.event_type === 'browser_info')
    .map((e) => ({
      id: `browser-info-${e.timestamp}`,
      timestamp: e.timestamp,
      captured: e.captured,
      success: e.success,
      message: e.message,
      user_agent: e.user_agent,
      cdp_url: e.cdp_url,
      vnc_url: e.vnc_url,
      viewport_width: e.viewport_width,
      viewport_height: e.viewport_height,
    }));
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
   * updates adjacent (e.g., task_complete deltas) while preserving older, non-adjacent
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
