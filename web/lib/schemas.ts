// Zod schemas for runtime validation of SSE events
// Aligns with the workflow-first envelope defined in ./types

import { z } from 'zod';
import type { AnyAgentEvent } from './types';
import { WorkflowEventType } from './types';

const AgentLevelSchema = z.enum(['core', 'subagent']);

const BaseAgentEventSchema = z.object({
  version: z.number().optional().default(1),
  event_type: z.string(),
  timestamp: z.string().default(() => new Date().toISOString()),
  agent_level: AgentLevelSchema.default('core'),
  session_id: z.string().default(''),
  task_id: z.string().optional(),
  parent_task_id: z.string().optional(),
  workflow_id: z.string().optional(),
  run_id: z.string().optional(),
  node_id: z.string().optional(),
  node_kind: z.string().optional(),
  is_subtask: z.boolean().optional(),
  subtask_index: z.number().optional(),
  total_subtasks: z.number().optional(),
  subtask_preview: z.string().optional(),
  max_parallel: z.number().optional(),
  payload: z.record(z.string(), z.unknown()).nullable().optional(),
});

export const AttachmentPreviewAssetPayloadSchema = z.object({
  asset_id: z.string().optional(),
  label: z.string().optional(),
  mime_type: z.string().optional(),
  cdn_url: z.string().optional(),
  preview_type: z.string().optional(),
});

export const AttachmentPayloadSchema = z.object({
  name: z.string(),
  media_type: z.string(),
  data: z.string().optional(),
  uri: z.string().optional(),
  source: z.string().optional(),
  description: z.string().optional(),
  kind: z.string().optional(),
  format: z.string().optional(),
  preview_profile: z.string().optional(),
  preview_assets: z.array(AttachmentPreviewAssetPayloadSchema).optional(),
  retention_ttl_seconds: z.number().optional(),
});

export const ToolCallSchema = z.object({
  id: z.string(),
  name: z.string(),
  arguments: z.record(z.string(), z.any()),
  session_id: z.string().optional(),
  task_id: z.string().optional(),
  parent_task_id: z.string().optional(),
});

export const ToolResultSchema = z.object({
  call_id: z.string(),
  content: z.string(),
  error: z.string().optional(),
  metadata: z.record(z.string(), z.any()).optional(),
  attachments: z.record(z.string(), AttachmentPayloadSchema).nullable().optional(),
});

export const MessageSchema = z.object({
  role: z.string(),
  content: z.string(),
  tool_calls: z.array(ToolCallSchema).optional(),
  tool_results: z.array(ToolResultSchema).optional(),
  tool_call_id: z.string().optional(),
  metadata: z.record(z.string(), z.any()).optional(),
  attachments: z.record(z.string(), AttachmentPayloadSchema).nullable().optional(),
  source: z.string().optional(),
});

export const WorkflowNodeSnapshotSchema = z.object({
  id: z.string(),
  status: z.enum(['pending', 'running', 'succeeded', 'failed']),
  kind: z.string().optional(),
  input: z.any().optional(),
  output: z.any().optional(),
  error: z.string().optional(),
  started_at: z.string().optional(),
  completed_at: z.string().optional(),
  duration: z.number().optional(),
});

export const WorkflowSnapshotSchema = z.object({
  id: z.string(),
  phase: z.enum(['pending', 'running', 'succeeded', 'failed']),
  order: z.array(z.string()),
  nodes: z.array(WorkflowNodeSnapshotSchema).optional(),
  started_at: z.string().optional(),
  completed_at: z.string().optional(),
  duration: z.number().optional(),
  summary: z.record(z.string(), z.number()),
});

const WorkflowLifecycleUpdatedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.lifecycle.updated'),
  workflow_id: z.string().optional(),
  workflow_event_type: z.enum(['node_added', 'node_started', 'node_succeeded', 'node_failed', 'workflow_updated']),
  phase: z.enum(['pending', 'running', 'succeeded', 'failed']).optional(),
  node: WorkflowNodeSnapshotSchema.optional(),
  workflow: WorkflowSnapshotSchema.optional(),
});

const WorkflowNodeStartedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.node.started'),
  node_id: z.string().optional(),
  node_kind: z.string().optional(),
  step_index: z.number().optional(),
  step_description: z.string().optional(),
  iteration: z.number().optional(),
  total_iters: z.number().optional(),
  workflow: WorkflowSnapshotSchema.optional(),
});

const WorkflowNodeCompletedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.node.completed'),
  node_id: z.string().optional(),
  node_kind: z.string().optional(),
  step_index: z.number().optional(),
  step_description: z.string().optional(),
  step_result: z.any().optional(),
  status: z.enum(['pending', 'running', 'succeeded', 'failed']).optional(),
  iteration: z.number().optional(),
  tokens_used: z.number().optional(),
  tools_run: z.number().optional(),
  workflow: WorkflowSnapshotSchema.optional(),
});

const WorkflowNodeFailedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.node.failed'),
  node_id: z.string().optional(),
  node_kind: z.string().optional(),
  iteration: z.number().optional(),
  error: z.string(),
  phase: z.string().optional(),
  recoverable: z.boolean().optional(),
});

const WorkflowNodeOutputDeltaEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.node.output.delta'),
  node_id: z.string().optional(),
  node_kind: z.string().optional(),
  iteration: z.number().optional(),
  delta: z.string().default(''),
  final: z.boolean().optional().default(false),
  created_at: z.string().optional(),
  source_model: z.string().optional(),
  message_count: z.number().optional(),
});

const WorkflowNodeOutputSummaryEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.node.output.summary'),
  node_id: z.string().optional(),
  node_kind: z.string().optional(),
  iteration: z.number().optional(),
  content: z.string(),
  tool_call_count: z.number(),
  attachments: z.record(z.string(), AttachmentPayloadSchema).nullable().optional(),
});

const WorkflowToolStartedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.tool.started'),
  call_id: z.string(),
  tool_name: z.string(),
  arguments: z.record(z.string(), z.unknown()).default({}),
  arguments_preview: z.string().optional(),
  iteration: z.number().optional(),
});

const WorkflowToolProgressEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.tool.progress'),
  call_id: z.string(),
  chunk: z.string().default(''),
  is_complete: z.boolean().optional(),
});

const WorkflowToolCompletedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.tool.completed'),
  call_id: z.string(),
  tool_name: z.string(),
  result: z.string().default(''),
  error: z.string().optional(),
  duration: z.number().default(0),
  metadata: z.record(z.string(), z.unknown()).optional(),
  attachments: z.record(z.string(), AttachmentPayloadSchema).nullable().optional(),
});

const WorkflowResultFinalEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.result.final'),
  final_answer: z.string().default(''),
  total_iterations: z.number().default(0),
  total_tokens: z.number().default(0),
  stop_reason: z.string().default('complete'),
  duration: z.number().default(0),
  is_streaming: z.boolean().optional(),
  stream_finished: z.boolean().optional(),
  attachments: z.record(z.string(), AttachmentPayloadSchema).nullable().optional(),
});

const WorkflowResultCancelledEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.result.cancelled'),
  reason: z.string().default('cancelled'),
  requested_by: z.enum(['user', 'system']).optional(),
});

const WorkflowSubflowProgressEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.subflow.progress'),
  completed: z.number().default(0),
  total: z.number().default(0),
  tokens: z.number().default(0),
  tool_calls: z.number().default(0),
});

const WorkflowSubflowCompletedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.subflow.completed'),
  total: z.number().default(0),
  success: z.number().default(0),
  failed: z.number().default(0),
  tokens: z.number().default(0),
  tool_calls: z.number().default(0),
});

const WorkflowDiagnosticEnvironmentSnapshotEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.diagnostic.environment_snapshot'),
  host: z.record(z.string(), z.string()).nullable().optional(),
  captured: z.string(),
});

const WorkflowDiagnosticContextCompressionEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.diagnostic.context_compression'),
  original_count: z.number(),
  compressed_count: z.number(),
  compression_rate: z.number(),
});

const WorkflowDiagnosticToolFilteringEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.diagnostic.tool_filtering'),
  preset_name: z.string(),
  original_count: z.number(),
  filtered_count: z.number(),
  filtered_tools: z.array(z.string()),
  tool_filter_ratio: z.number(),
});

const WorkflowDiagnosticContextSnapshotEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.diagnostic.context_snapshot'),
  iteration: z.number(),
  llm_turn_seq: z.number(),
  request_id: z.string(),
  messages: z.array(MessageSchema),
  excluded_messages: z.array(MessageSchema).optional(),
});

const ConnectedEventSchema = z.object({
  event_type: z.literal('connected'),
  session_id: z.string(),
  task_id: z.string().optional(),
  parent_task_id: z.string().optional(),
  timestamp: z.string().optional(),
  agent_level: AgentLevelSchema.optional(),
});

const WorkflowDiagnosticErrorEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.diagnostic.error'),
  iteration: z.number().optional(),
  phase: z.string().optional(),
  recoverable: z.boolean().optional(),
  error: z.string().optional(),
});

const WorkflowInputReceivedEventSchema = BaseAgentEventSchema.extend({
  event_type: z.literal('workflow.input.received'),
  task: z.string(),
  attachments: z.record(z.string(), AttachmentPayloadSchema).nullable().optional(),
});

const EVENT_TYPE_ALIASES: Record<string, WorkflowEventType> = {};

const EventSchemas = [
  WorkflowLifecycleUpdatedEventSchema,
  WorkflowNodeStartedEventSchema,
  WorkflowNodeCompletedEventSchema,
  WorkflowNodeFailedEventSchema,
  WorkflowNodeOutputDeltaEventSchema,
  WorkflowNodeOutputSummaryEventSchema,
  WorkflowToolStartedEventSchema,
  WorkflowToolProgressEventSchema,
  WorkflowToolCompletedEventSchema,
  WorkflowResultFinalEventSchema,
  WorkflowResultCancelledEventSchema,
  WorkflowSubflowProgressEventSchema,
  WorkflowSubflowCompletedEventSchema,
  WorkflowDiagnosticEnvironmentSnapshotEventSchema,
  WorkflowDiagnosticContextCompressionEventSchema,
  WorkflowDiagnosticToolFilteringEventSchema,
  WorkflowDiagnosticContextSnapshotEventSchema,
  WorkflowDiagnosticErrorEventSchema,
  ConnectedEventSchema,
  WorkflowInputReceivedEventSchema,
] as const;

const eventSchemaList = [...EventSchemas] as [
  (typeof EventSchemas)[number],
  ...(typeof EventSchemas)[number][],
];

export const AnyAgentEventSchema = z.discriminatedUnion(
  'event_type',
  eventSchemaList,
);

function normalizeEventData(data: unknown): Record<string, any> | null {
  if (typeof data !== 'object' || data === null) {
    return null;
  }

  const event = data as Record<string, any>;
  const payload = event.payload;
  const payloadObject =
    payload && typeof payload === 'object' && !Array.isArray(payload) ? (payload as Record<string, any>) : null;
  const normalized: Record<string, any> = payloadObject ? { ...payloadObject, ...event } : { ...event };
  normalized.payload = payloadObject ?? event.payload ?? null;

  if (payloadObject && normalized.final_answer === undefined && typeof payloadObject.final_answer === 'string') {
    normalized.final_answer = payloadObject.final_answer;
  }
  if (payloadObject && normalized.duration === undefined && typeof payloadObject.duration_ms === 'number') {
    normalized.duration = payloadObject.duration_ms;
  }

  if (!normalized.timestamp) {
    normalized.timestamp = new Date().toISOString();
  }
  if (normalized.agent_level === undefined) {
    normalized.agent_level = 'core';
  }
  if (normalized.session_id === undefined) {
    normalized.session_id = '';
  }
  if (normalized.version === undefined || normalized.version === null) {
    normalized.version = 1;
  }

  const rawEventType = normalized.event_type as string | undefined;
  const aliasTarget = rawEventType ? EVENT_TYPE_ALIASES[rawEventType] : undefined;
  if (aliasTarget) {
    normalized.event_type = aliasTarget;
  }

  if (!rawEventType) {
    return null;
  }

  return normalized;
}

export function safeValidateEvent(
  data: unknown,
): { success: true; data: AnyAgentEvent } | { success: false; error: z.ZodError; raw: unknown } {
  const normalized = normalizeEventData(data);
  if (!normalized) {
    return {
      success: false,
      error: new z.ZodError([
        {
          code: 'custom',
          path: ['event_type'],
          message: 'Event type could not be normalized',
        },
      ]),
      raw: data,
    };
  }

  const result = AnyAgentEventSchema.safeParse(normalized);
  if (result.success) {
    return { success: true, data: result.data as AnyAgentEvent };
  }

  return { success: false, error: result.error, raw: data };
}
