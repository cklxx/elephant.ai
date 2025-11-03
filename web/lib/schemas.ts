// Zod schemas for runtime validation of SSE events
// Corresponds to TypeScript types in lib/types.ts

import { z } from 'zod';

// Base schema for all agent events
export const AgentLevelSchema = z.enum(['core', 'subagent']);

export const BaseAgentEventSchema = z.object({
  event_type: z.string(),
  timestamp: z.string(),
  agent_level: AgentLevelSchema,
  session_id: z.string(),
  task_id: z.string().optional(),
  parent_task_id: z.string().optional(),
});

// Task Analysis Event
export const TaskAnalysisEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('task_analysis'),
  action_name: z.string(),
  goal: z.string(),
});

// Iteration Start Event
export const IterationStartEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('iteration_start'),
  iteration: z.number(),
  total_iters: z.number(),
});

// Thinking Event
export const ThinkingEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('thinking'),
  iteration: z.number(),
  message_count: z.number(),
});

// Think Complete Event
export const ThinkCompleteEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('think_complete'),
  iteration: z.number(),
  content: z.string(),
  tool_call_count: z.number(),
});

// Tool Call Start Event
export const ToolCallStartEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('tool_call_start'),
  iteration: z.number(),
  call_id: z.string(),
  tool_name: z.string(),
  arguments: z.record(z.string(), z.any()),
  arguments_preview: z.string().optional(),
});

// Tool Call Stream Event
export const ToolCallStreamEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('tool_call_stream'),
  call_id: z.string(),
  chunk: z.string(),
  is_complete: z.boolean(),
});

// Tool Call Complete Event
export const ToolCallCompleteEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('tool_call_complete'),
  call_id: z.string(),
  tool_name: z.string(),
  result: z.string(),
  error: z.string().optional(),
  duration: z.number(),
  metadata: z.record(z.string(), z.any()).optional(),
});

// Iteration Complete Event
export const IterationCompleteEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('iteration_complete'),
  iteration: z.number(),
  tokens_used: z.number(),
  tools_run: z.number(),
});

// Task Complete Event
export const TaskCompleteEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('task_complete'),
  final_answer: z.string(),
  total_iterations: z.number(),
  total_tokens: z.number(),
  stop_reason: z.string(),
  duration: z.number(),
});

// Error Event
export const ErrorEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('error'),
  iteration: z.number(),
  phase: z.string(),
  error: z.string(),
  recoverable: z.boolean(),
});

// Research Plan Event
export const ResearchPlanEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('research_plan'),
  plan_steps: z.array(z.string()),
  estimated_iterations: z.number(),
  estimated_tools: z.array(z.string()).optional(),
  estimated_duration_minutes: z.number().optional(),
});

// Step Started Event
export const StepStartedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('step_started'),
  step_index: z.number(),
  step_description: z.string(),
  iteration: z.number().optional(),
});

// Step Completed Event
export const StepCompletedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('step_completed'),
  step_index: z.number(),
  step_result: z.string(),
  iteration: z.number().optional(),
  step_description: z.string().optional(),
});

// Browser Info Event
export const BrowserInfoEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('browser_info'),
  success: z.boolean().optional(),
  message: z.string().optional(),
  user_agent: z.string().optional(),
  cdp_url: z.string().optional(),
  vnc_url: z.string().optional(),
  viewport_width: z.number().optional(),
  viewport_height: z.number().optional(),
  captured: z.string(),
});

// Environment Snapshot Event
export const EnvironmentSnapshotEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('environment_snapshot'),
  host: z.record(z.string(), z.string()).nullable().optional(),
  sandbox: z.record(z.string(), z.string()).nullable().optional(),
  captured: z.string(),
});

// Sandbox Progress Event
export const SandboxProgressEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('sandbox_progress'),
  status: z.enum(['pending', 'running', 'ready', 'error']),
  stage: z.string(),
  message: z.string().optional(),
  step: z.number(),
  total_steps: z.number(),
  error: z.string().optional(),
  updated: z.string(),
});

// Context Compression Event
export const ContextCompressionEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('context_compression'),
  original_count: z.number(),
  compressed_count: z.number(),
  compression_rate: z.number(),
});

// Tool Filtering Event
export const ToolFilteringEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('tool_filtering'),
  preset_name: z.string(),
  original_count: z.number(),
  filtered_count: z.number(),
  filtered_tools: z.array(z.string()),
  tool_filter_ratio: z.number(),
});

// Connected Event
export const ConnectedEventSchema = z.object({
  event_type: z.literal('connected'),
  session_id: z.string(),
  task_id: z.string().optional(),
  parent_task_id: z.string().optional(),
  timestamp: z.string().optional(),
  agent_level: AgentLevelSchema.optional(),
});

// User Task Event (client-side only)
export const UserTaskEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('user_task'),
  task: z.string(),
});

// Union schema for all agent events
export const AnyAgentEventSchema = z.discriminatedUnion('event_type', [
  TaskAnalysisEventSchema,
  IterationStartEventSchema,
  ThinkingEventSchema,
  ThinkCompleteEventSchema,
  ToolCallStartEventSchema,
  ToolCallStreamEventSchema,
  ToolCallCompleteEventSchema,
  IterationCompleteEventSchema,
  TaskCompleteEventSchema,
  ErrorEventSchema,
  ResearchPlanEventSchema,
  StepStartedEventSchema,
  StepCompletedEventSchema,
  BrowserInfoEventSchema,
  EnvironmentSnapshotEventSchema,
  SandboxProgressEventSchema,
  ContextCompressionEventSchema,
  ToolFilteringEventSchema,
  ConnectedEventSchema,
  UserTaskEventSchema,
]);

// API Request/Response Schemas

export const CreateTaskRequestSchema = z.object({
  task: z.string(),
  session_id: z.string().optional(),
  auto_approve_plan: z.boolean().optional(),
});

export const CreateTaskResponseSchema = z.object({
  task_id: z.string(),
  session_id: z.string(),
  parent_task_id: z.string().optional(),
  status: z.enum(['pending', 'running', 'completed', 'failed']),
  requires_plan_approval: z.boolean().optional(),
});

export const ResearchPlanSchema = z.object({
  goal: z.string(),
  steps: z.array(z.string()),
  estimated_tools: z.array(z.string()),
  estimated_iterations: z.number(),
});

export const ApprovePlanRequestSchema = z.object({
  session_id: z.string(),
  task_id: z.string(),
  approved: z.boolean(),
  modified_plan: ResearchPlanSchema.optional(),
});

export const ApprovePlanResponseSchema = z.object({
  success: z.boolean(),
  message: z.string(),
});

export const TaskStatusResponseSchema = z.object({
  task_id: z.string(),
  session_id: z.string(),
  parent_task_id: z.string().optional(),
  status: z.enum(['pending', 'running', 'completed', 'failed']),
  created_at: z.string(),
  completed_at: z.string().optional(),
  error: z.string().optional(),
});

export const SessionSchema = z.object({
  id: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
  task_count: z.number(),
  last_task: z.string().optional(),
});

export const SessionListResponseSchema = z.object({
  sessions: z.array(SessionSchema),
  total: z.number(),
});

export const SessionDetailsResponseSchema = z.object({
  session: SessionSchema,
  tasks: z.array(TaskStatusResponseSchema),
});

// Helper function to normalize and complete event data
function normalizeEventData(data: unknown): unknown {
  if (typeof data !== 'object' || data === null) {
    return data;
  }

  const event = data as Record<string, any>;
  const normalized: Record<string, any> = { ...event };

  // Ensure timestamp exists
  if (!normalized.timestamp) {
    normalized.timestamp = new Date().toISOString();
  }

  // Ensure agent_level exists
  if (!normalized.agent_level) {
    normalized.agent_level = 'core';
  }

  // Ensure session_id exists
  if (!normalized.session_id) {
    normalized.session_id = '';
  }

  // For tool_call_start events, ensure arguments exists
  if (normalized.event_type === 'tool_call_start' && !normalized.arguments) {
    normalized.arguments = {};
  }

  // Handle missing event_type - try to infer or skip
  if (!normalized.event_type || typeof normalized.event_type !== 'string') {
    // Try to infer event type from available fields
    if ('final_answer' in normalized && 'total_iterations' in normalized) {
      normalized.event_type = 'task_complete';
    } else if ('tool_name' in normalized && 'result' in normalized && 'call_id' in normalized) {
      normalized.event_type = 'tool_call_complete';
    } else if ('tool_name' in normalized && 'call_id' in normalized && !('result' in normalized)) {
      normalized.event_type = 'tool_call_start';
    } else if ('call_id' in normalized && 'chunk' in normalized) {
      normalized.event_type = 'tool_call_stream';
    } else if ('content' in normalized && 'tool_call_count' in normalized) {
      normalized.event_type = 'think_complete';
    } else if ('iteration' in normalized && 'tokens_used' in normalized) {
      normalized.event_type = 'iteration_complete';
    } else if ('iteration' in normalized && 'message_count' in normalized) {
      normalized.event_type = 'thinking';
    } else if ('iteration' in normalized && 'total_iters' in normalized) {
      normalized.event_type = 'iteration_start';
    } else if ('action_name' in normalized && 'goal' in normalized) {
      normalized.event_type = 'task_analysis';
    } else if ('plan_steps' in normalized) {
      normalized.event_type = 'research_plan';
    } else if ('step_index' in normalized && 'step_description' in normalized && !('step_result' in normalized)) {
      normalized.event_type = 'step_started';
    } else if ('step_index' in normalized && 'step_result' in normalized) {
      normalized.event_type = 'step_completed';
    } else if ('phase' in normalized && 'recoverable' in normalized) {
      normalized.event_type = 'error';
    } else if ('status' in normalized && 'stage' in normalized && 'step' in normalized) {
      normalized.event_type = 'sandbox_progress';
    } else if ('user_agent' in normalized || 'cdp_url' in normalized) {
      normalized.event_type = 'browser_info';
    } else if ('host' in normalized || 'sandbox' in normalized) {
      normalized.event_type = 'environment_snapshot';
    } else if ('original_count' in normalized && 'compressed_count' in normalized) {
      normalized.event_type = 'context_compression';
    } else if ('preset_name' in normalized && 'filtered_tools' in normalized) {
      normalized.event_type = 'tool_filtering';
    } else {
      // Cannot infer - log and skip this event
      console.debug('[Schema Normalization] Cannot infer event_type, available fields:', Object.keys(normalized));
      return null;
    }
  }

  return normalized;
}

// Helper function to validate and parse events
export function validateEvent(data: unknown): z.infer<typeof AnyAgentEventSchema> | null {
  try {
    // First try to normalize the data
    const normalized = normalizeEventData(data);
    if (normalized === null) {
      console.warn('[Schema Validation] Could not normalize event, skipping:', data);
      return null;
    }

    return AnyAgentEventSchema.parse(normalized);
  } catch (error) {
    console.error('[Schema Validation] Failed to validate event:', error);
    console.debug('[Schema Validation] Raw event data:', data);
    return null;
  }
}

// Helper function to safely validate without throwing
export function safeValidateEvent(data: unknown): { success: true; data: z.infer<typeof AnyAgentEventSchema> } | { success: false; error: z.ZodError; raw: unknown } {
  // First try to normalize the data
  const normalized = normalizeEventData(data);

  if (normalized === null) {
    console.warn('[Schema Validation] Could not normalize event, skipping:', data);
    // Return a synthetic error for skipped events
    return {
      success: false,
      error: new z.ZodError([{
        code: 'custom',
        path: ['event_type'],
        message: 'Event type could not be inferred',
      }]),
      raw: data
    };
  }

  const result = AnyAgentEventSchema.safeParse(normalized);
  if (result.success) {
    return { success: true, data: result.data };
  }

  return { success: false, error: result.error, raw: data };
}
