// Zod schemas for runtime validation of SSE events
// Corresponds to TypeScript types in lib/types.ts

import { z } from 'zod';

// Base schema for all agent events
export const AgentLevelSchema = z.enum(['core', 'subagent']);

export const BaseAgentEventSchema = z.object({
  event_type: z.string(),
  timestamp: z.string(),
  agent_level: AgentLevelSchema,
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

// Browser Snapshot Event
export const BrowserSnapshotEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('browser_snapshot'),
  url: z.string(),
  screenshot_data: z.string().optional(),
  html_preview: z.string().optional(),
});

// Connected Event
export const ConnectedEventSchema = z.object({
  event_type: z.literal('connected'),
  session_id: z.string(),
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
  BrowserSnapshotEventSchema,
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

// Helper function to validate and parse events
export function validateEvent(data: unknown): z.infer<typeof AnyAgentEventSchema> | null {
  try {
    return AnyAgentEventSchema.parse(data);
  } catch (error) {
    console.error('[Schema Validation] Failed to validate event:', error);
    return null;
  }
}

// Helper function to safely validate without throwing
export function safeValidateEvent(data: unknown): { success: true; data: z.infer<typeof AnyAgentEventSchema> } | { success: false; error: z.ZodError } {
  const result = AnyAgentEventSchema.safeParse(data);
  if (result.success) {
    return { success: true, data: result.data };
  }
  return { success: false, error: result.error };
}
