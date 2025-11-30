// TypeScript types for ALEX Web Frontend
// Workflow-first event envelope with semantic, namespaced event_type values.

export type AgentLevel = 'core' | 'subagent';

export type WorkflowPhase = 'pending' | 'running' | 'succeeded' | 'failed';
export type WorkflowNodeStatus = 'pending' | 'running' | 'succeeded' | 'failed';

// Legacy workflow transition values emitted inside workflow lifecycle snapshots.
export type WorkflowLifecycleEventType =
  | 'node_added'
  | 'node_started'
  | 'node_succeeded'
  | 'node_failed'
  | 'workflow_updated';

export type WorkflowEventType =
  | 'workflow.lifecycle.updated'
  | 'workflow.plan.generated'
  | 'workflow.node.started'
  | 'workflow.node.completed'
  | 'workflow.node.failed'
  | 'workflow.node.output.delta'
  | 'workflow.node.output.summary'
  | 'workflow.tool.started'
  | 'workflow.tool.progress'
  | 'workflow.tool.completed'
  | 'workflow.subflow.progress'
  | 'workflow.subflow.completed'
  | 'workflow.result.final'
  | 'workflow.result.cancelled'
  | 'workflow.diagnostic.context_compression'
  | 'workflow.diagnostic.tool_filtering'
  | 'workflow.diagnostic.browser_info'
  | 'workflow.diagnostic.environment_snapshot'
  | 'workflow.diagnostic.sandbox_progress'
  | 'workflow.diagnostic.context_snapshot';

export type LegacyEventType =
  | 'workflow_event'
  | 'iteration_start'
  | 'thinking'
  | 'assistant_message'
  | 'think_complete'
  | 'tool_call_start'
  | 'tool_call_stream'
  | 'tool_call_complete'
  | 'iteration_complete'
  | 'subagent_progress'
  | 'task_cancelled'
  | 'task_complete'
  | 'error'
  | 'subagent_complete'
  | 'research_plan'
  | 'step_started'
  | 'step_completed'
  | 'browser_info'
  | 'environment_snapshot'
  | 'sandbox_progress'
  | 'context_compression'
  | 'tool_filtering'
  | 'context_snapshot';

export const LegacyEventTypeToWorkflowType: Record<LegacyEventType, WorkflowEventType> = {
  workflow_event: 'workflow.lifecycle.updated',
  iteration_start: 'workflow.node.started',
  thinking: 'workflow.node.output.delta',
  assistant_message: 'workflow.node.output.delta',
  think_complete: 'workflow.node.output.summary',
  tool_call_start: 'workflow.tool.started',
  tool_call_stream: 'workflow.tool.progress',
  tool_call_complete: 'workflow.tool.completed',
  iteration_complete: 'workflow.node.completed',
  subagent_progress: 'workflow.subflow.progress',
  task_cancelled: 'workflow.result.cancelled',
  task_complete: 'workflow.result.final',
  error: 'workflow.node.failed',
  subagent_complete: 'workflow.subflow.completed',
  research_plan: 'workflow.plan.generated',
  step_started: 'workflow.node.started',
  step_completed: 'workflow.node.completed',
  browser_info: 'workflow.diagnostic.browser_info',
  environment_snapshot: 'workflow.diagnostic.environment_snapshot',
  sandbox_progress: 'workflow.diagnostic.sandbox_progress',
  context_compression: 'workflow.diagnostic.context_compression',
  tool_filtering: 'workflow.diagnostic.tool_filtering',
  context_snapshot: 'workflow.diagnostic.context_snapshot',
};

export type AgentEventType = WorkflowEventType | LegacyEventType | 'connected' | 'user_task';

export interface WorkflowNodeSnapshot {
  id: string;
  status: WorkflowNodeStatus;
  kind?: string;
  input?: any;
  output?: any;
  error?: string;
  started_at?: string;
  completed_at?: string;
  duration?: number;
}

export interface WorkflowSnapshot {
  id: string;
  phase: WorkflowPhase;
  order: string[];
  nodes: WorkflowNodeSnapshot[];
  started_at?: string;
  completed_at?: string;
  duration?: number;
  summary: Record<WorkflowNodeStatus, number>;
}

export interface AttachmentPreviewAssetPayload {
  asset_id?: string;
  label?: string;
  mime_type?: string;
  cdn_url?: string;
  preview_type?: string;
}

export interface AttachmentPayload {
  name: string;
  media_type: string;
  data?: string;
  uri?: string;
  source?: string;
  description?: string;
  kind?: 'attachment' | 'artifact' | string;
  format?: string;
  preview_profile?: string;
  preview_assets?: AttachmentPreviewAssetPayload[];
  retention_ttl_seconds?: number;
}

export interface AttachmentUpload {
  name: string;
  media_type: string;
  data?: string;
  uri?: string;
  source?: string;
  description?: string;
  kind?: 'attachment' | 'artifact' | string;
  format?: string;
  retention_ttl_seconds?: number;
}

export type MessageSource =
  | 'system_prompt'
  | 'user_input'
  | 'user_history'
  | 'assistant_reply'
  | 'tool_result'
  | 'debug'
  | 'evaluation';

export interface ToolCall {
  id: string;
  name: string;
  arguments: Record<string, any>;
  session_id?: string;
  task_id?: string;
  parent_task_id?: string;
}

export interface ToolResult {
  call_id: string;
  content: string;
  error?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
}

export interface Message {
  role: string;
  content: string;
  tool_calls?: ToolCall[];
  tool_results?: ToolResult[];
  tool_call_id?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
  source?: MessageSource;
}

export interface WorkflowEnvelope<TType extends AgentEventType = WorkflowEventType> {
  version?: number;
  event_type: TType;
  timestamp: string;
  agent_level: AgentLevel;
  session_id: string;
  task_id?: string;
  parent_task_id?: string;
  workflow_id?: string;
  run_id?: string;
  node_id?: string;
  node_kind?: string;
  legacy_type?: LegacyEventType;
  is_subtask?: boolean;
  subtask_index?: number;
  total_subtasks?: number;
  subtask_preview?: string;
  max_parallel?: number;
}

export type WorkflowEvent<Payload, Type extends AgentEventType = WorkflowEventType> = WorkflowEnvelope<Type> &
  Payload & {
    payload?: Payload;
  };

// Compatibility alias for generic agent events
export type AgentEvent<TPayload = Record<string, unknown>> = WorkflowEnvelope & TPayload;

// Backwards-compatible alias for historical base event interface
export type AgentEvent<TPayload = Record<string, unknown>> = WorkflowEnvelope<AgentEventType> & TPayload;

export interface WorkflowLifecycleUpdatedPayload {
  workflow_id?: string;
  workflow_event_type: WorkflowLifecycleEventType;
  phase?: WorkflowPhase;
  node?: WorkflowNodeSnapshot;
  workflow?: WorkflowSnapshot;
}

export interface WorkflowPlanGeneratedPayload {
  plan_steps: string[];
  estimated_iterations: number;
  estimated_tools?: string[];
  estimated_duration_minutes?: number;
}

export interface WorkflowNodeStartedPayload {
  node_id?: string;
  node_kind?: string;
  step_index?: number;
  step_description?: string;
  iteration?: number;
  total_iters?: number;
  workflow?: WorkflowSnapshot;
}

export interface WorkflowNodeCompletedPayload {
  node_id?: string;
  node_kind?: string;
  step_index?: number;
  step_description?: string;
  step_result?: any;
  status?: WorkflowNodeStatus;
  iteration?: number;
  tokens_used?: number;
  tools_run?: number;
  workflow?: WorkflowSnapshot;
}

export interface WorkflowNodeFailedPayload {
  node_id?: string;
  node_kind?: string;
  iteration?: number;
  error: string;
  phase?: string;
  recoverable?: boolean;
}

export interface WorkflowNodeOutputDeltaPayload {
  node_id?: string;
  node_kind?: string;
  iteration?: number;
  delta?: string;
  final?: boolean;
  created_at?: string;
  source_model?: string;
  message_count?: number;
}

export interface WorkflowNodeOutputSummaryPayload {
  node_id?: string;
  node_kind?: string;
  iteration?: number;
  content: string;
  tool_call_count: number;
  attachments?: Record<string, AttachmentPayload>;
}

export interface WorkflowToolStartedPayload {
  call_id: string;
  tool_name: string;
  arguments: Record<string, any>;
  arguments_preview?: string;
  iteration?: number;
}

export interface WorkflowToolProgressPayload {
  call_id: string;
  chunk: string;
  is_complete?: boolean;
}

export interface WorkflowToolCompletedPayload {
  call_id: string;
  tool_name: string;
  result: string;
  error?: string;
  duration: number;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
}

export interface WorkflowResultFinalPayload {
  final_answer: string;
  total_iterations: number;
  total_tokens: number;
  stop_reason: string;
  duration: number;
  is_streaming?: boolean;
  stream_finished?: boolean;
  attachments?: Record<string, AttachmentPayload>;
}

export interface WorkflowResultCancelledPayload {
  reason: string;
  requested_by?: 'user' | 'system';
}

export interface WorkflowSubflowProgressPayload {
  completed: number;
  total: number;
  tokens: number;
  tool_calls: number;
}

export interface WorkflowSubflowCompletedPayload {
  total: number;
  success: number;
  failed: number;
  tokens: number;
  tool_calls: number;
}

export interface WorkflowDiagnosticBrowserInfoPayload {
  success?: boolean;
  message?: string;
  user_agent?: string;
  cdp_url?: string;
  vnc_url?: string;
  viewport_width?: number;
  viewport_height?: number;
  captured: string;
}

export interface WorkflowDiagnosticEnvironmentSnapshotPayload {
  host?: Record<string, string> | null;
  sandbox?: Record<string, string> | null;
  captured: string;
}

export type SandboxProgressStatus = 'pending' | 'running' | 'ready' | 'error';

export interface WorkflowDiagnosticSandboxProgressPayload {
  status: SandboxProgressStatus;
  stage: string;
  message?: string;
  step: number;
  total_steps: number;
  error?: string;
  updated: string;
}

export interface WorkflowDiagnosticContextCompressionPayload {
  original_count: number;
  compressed_count: number;
  compression_rate: number;
}

export interface WorkflowDiagnosticToolFilteringPayload {
  preset_name: string;
  original_count: number;
  filtered_count: number;
  filtered_tools: string[];
  tool_filter_ratio: number;
}

export interface WorkflowDiagnosticContextSnapshotPayload {
  iteration: number;
  llm_turn_seq: number;
  request_id: string;
  messages: Message[];
  excluded_messages?: Message[];
}

export interface UserTaskPayload {
  task: string;
  attachments?: Record<string, AttachmentPayload>;
}

export type WorkflowLifecycleUpdatedEvent = WorkflowEvent<
  WorkflowLifecycleUpdatedPayload,
  'workflow.lifecycle.updated' | 'workflow_event'
>;
export type WorkflowPlanGeneratedEvent = WorkflowEvent<
  WorkflowPlanGeneratedPayload,
  'workflow.plan.generated' | 'research_plan'
>;
export type WorkflowNodeStartedEvent = WorkflowEvent<
  WorkflowNodeStartedPayload,
  'workflow.node.started' | 'step_started' | 'iteration_start'
>;
export type WorkflowNodeCompletedEvent = WorkflowEvent<
  WorkflowNodeCompletedPayload,
  'workflow.node.completed' | 'step_completed' | 'iteration_complete'
>;
export type WorkflowNodeFailedEvent = WorkflowEvent<
  WorkflowNodeFailedPayload,
  'workflow.node.failed' | 'error'
>;
export type WorkflowNodeOutputDeltaEvent = WorkflowEvent<
  WorkflowNodeOutputDeltaPayload,
  'workflow.node.output.delta' | 'assistant_message' | 'thinking'
>;
export type WorkflowNodeOutputSummaryEvent = WorkflowEvent<
  WorkflowNodeOutputSummaryPayload,
  'workflow.node.output.summary' | 'think_complete'
>;
export type WorkflowToolStartedEvent = WorkflowEvent<
  WorkflowToolStartedPayload,
  'workflow.tool.started' | 'tool_call_start'
>;
export type WorkflowToolProgressEvent = WorkflowEvent<
  WorkflowToolProgressPayload,
  'workflow.tool.progress' | 'tool_call_stream'
>;
export type WorkflowToolCompletedEvent = WorkflowEvent<
  WorkflowToolCompletedPayload,
  'workflow.tool.completed' | 'tool_call_complete'
>;
export type WorkflowResultFinalEvent = WorkflowEvent<
  WorkflowResultFinalPayload,
  'workflow.result.final' | 'task_complete'
>;
export type WorkflowResultCancelledEvent = WorkflowEvent<
  WorkflowResultCancelledPayload,
  'workflow.result.cancelled' | 'task_cancelled'
>;
export type WorkflowSubflowProgressEvent = WorkflowEvent<
  WorkflowSubflowProgressPayload,
  'workflow.subflow.progress' | 'subagent_progress'
>;
export type WorkflowSubflowCompletedEvent = WorkflowEvent<
  WorkflowSubflowCompletedPayload,
  'workflow.subflow.completed' | 'subagent_complete'
>;
export type WorkflowDiagnosticBrowserInfoEvent = WorkflowEvent<
  WorkflowDiagnosticBrowserInfoPayload,
  'workflow.diagnostic.browser_info' | 'browser_info'
>;
export type WorkflowDiagnosticEnvironmentSnapshotEvent = WorkflowEvent<
  WorkflowDiagnosticEnvironmentSnapshotPayload,
  'workflow.diagnostic.environment_snapshot' | 'environment_snapshot'
>;
export type WorkflowDiagnosticSandboxProgressEvent = WorkflowEvent<
  WorkflowDiagnosticSandboxProgressPayload,
  'workflow.diagnostic.sandbox_progress' | 'sandbox_progress'
>;
export type WorkflowDiagnosticContextCompressionEvent = WorkflowEvent<
  WorkflowDiagnosticContextCompressionPayload,
  'workflow.diagnostic.context_compression' | 'context_compression'
>;
export type WorkflowDiagnosticToolFilteringEvent = WorkflowEvent<
  WorkflowDiagnosticToolFilteringPayload,
  'workflow.diagnostic.tool_filtering' | 'tool_filtering'
>;
export type WorkflowDiagnosticContextSnapshotEvent = WorkflowEvent<
  WorkflowDiagnosticContextSnapshotPayload,
  'workflow.diagnostic.context_snapshot' | 'context_snapshot'
>;
export type UserTaskEvent = WorkflowEvent<UserTaskPayload, 'user_task'>;

export interface ConnectedEvent {
  event_type: 'connected';
  session_id: string;
  task_id?: string;
  parent_task_id?: string;
  timestamp?: string;
  agent_level?: AgentLevel;
}

// Backwards-compatible aliases for legacy names
export type IterationStartEvent = WorkflowNodeStartedEvent;
export type IterationCompleteEvent = WorkflowNodeCompletedEvent;
export type ThinkingEvent = WorkflowNodeOutputDeltaEvent;
export type AssistantMessageEvent = WorkflowNodeOutputDeltaEvent;
export type ThinkCompleteEvent = WorkflowNodeOutputSummaryEvent;
export type ToolCallStartEvent = WorkflowToolStartedEvent;
export type ToolCallStreamEvent = WorkflowToolProgressEvent;
export type ToolCallCompleteEvent = WorkflowToolCompletedEvent;
export type ResearchPlanEvent = WorkflowPlanGeneratedEvent;
export type StepStartedEvent = WorkflowNodeStartedEvent;
export type StepCompletedEvent = WorkflowNodeCompletedEvent;
export type TaskCompleteEvent = WorkflowResultFinalEvent;
export type TaskCancelledEvent = WorkflowResultCancelledEvent;
export type ErrorEvent = WorkflowNodeFailedEvent;
export type SubagentProgressEvent = WorkflowSubflowProgressEvent;
export type SubagentCompleteEvent = WorkflowSubflowCompletedEvent;
export type BrowserInfoEvent = WorkflowDiagnosticBrowserInfoEvent;
export type EnvironmentSnapshotEvent = WorkflowDiagnosticEnvironmentSnapshotEvent;
export type SandboxProgressEvent = WorkflowDiagnosticSandboxProgressEvent;
export type ContextCompressionEvent = WorkflowDiagnosticContextCompressionEvent;
export type ToolFilteringEvent = WorkflowDiagnosticToolFilteringEvent;
export type ContextSnapshotEvent = WorkflowDiagnosticContextSnapshotEvent;
export type WorkflowLifecycleEvent = WorkflowLifecycleUpdatedEvent;

export type LegacyAgentEvent = WorkflowEnvelope<LegacyEventType> &
  Record<string, any> & {
    payload?: Record<string, any>;
  };

export type AnyAgentEvent =
  | WorkflowLifecycleUpdatedEvent
  | WorkflowPlanGeneratedEvent
  | WorkflowNodeStartedEvent
  | WorkflowNodeCompletedEvent
  | WorkflowNodeFailedEvent
  | WorkflowNodeOutputDeltaEvent
  | WorkflowNodeOutputSummaryEvent
  | WorkflowToolStartedEvent
  | WorkflowToolProgressEvent
  | WorkflowToolCompletedEvent
  | WorkflowResultFinalEvent
  | WorkflowResultCancelledEvent
  | WorkflowSubflowProgressEvent
  | WorkflowSubflowCompletedEvent
  | WorkflowDiagnosticBrowserInfoEvent
  | WorkflowDiagnosticEnvironmentSnapshotEvent
  | WorkflowDiagnosticSandboxProgressEvent
  | WorkflowDiagnosticContextCompressionEvent
  | WorkflowDiagnosticToolFilteringEvent
  | WorkflowDiagnosticContextSnapshotEvent
  | ConnectedEvent
  | UserTaskEvent
  | LegacyAgentEvent;

export function canonicalEventType(
  event: Pick<WorkflowEnvelope, 'event_type'> & { legacy_type?: LegacyEventType },
): AgentEventType {
  if (event.event_type in LegacyEventTypeToWorkflowType) {
    return LegacyEventTypeToWorkflowType[event.event_type as LegacyEventType];
  }
  if (event.event_type === 'workflow.diagnostic.sandbox.progress') {
    return 'workflow.diagnostic.sandbox_progress';
  }
  if (event.legacy_type && event.legacy_type in LegacyEventTypeToWorkflowType) {
    return LegacyEventTypeToWorkflowType[event.legacy_type];
  }
  if (event.legacy_type === 'workflow.diagnostic.sandbox.progress') {
    return 'workflow.diagnostic.sandbox_progress';
  }
  return event.event_type;
}

export function eventMatches(
  event: AnyAgentEvent,
  ...types: (WorkflowEventType | LegacyEventType | 'connected' | 'user_task')[]
): boolean {
  const canonical = canonicalEventType(event);
  if (types.includes(canonical)) {
    return true;
  }
  if (types.includes(event.event_type)) {
    return true;
  }
  const legacyType = 'legacy_type' in event ? (event as any).legacy_type : undefined;
  if (legacyType && types.includes(legacyType)) {
    return true;
  }
  return false;
}
