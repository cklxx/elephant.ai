// TypeScript types for ALEX Web Frontend
// Workflow-first event envelope with semantic, namespaced event_type values.

export type AgentLevel = 'core' | 'subagent';

export type WorkflowPhase = 'pending' | 'running' | 'succeeded' | 'failed';
export type WorkflowNodeStatus = 'pending' | 'running' | 'succeeded' | 'failed';

export type WorkflowEventType =
  | 'workflow.lifecycle.updated'
  | 'workflow.node.started'
  | 'workflow.node.completed'
  | 'workflow.node.failed'
  | 'workflow.node.output.delta'
  | 'workflow.node.output.summary'
  | 'workflow.tool.started'
  | 'workflow.tool.progress'
  | 'workflow.tool.completed'
  | 'workflow.input.received'
  | 'workflow.subflow.progress'
  | 'workflow.subflow.completed'
  | 'workflow.result.final'
  | 'workflow.result.cancelled'
  | 'workflow.diagnostic.error'
  | 'workflow.diagnostic.context_compression'
  | 'workflow.diagnostic.tool_filtering'
  | 'workflow.diagnostic.environment_snapshot'
  | 'workflow.diagnostic.context_snapshot';

export type WorkflowLifecycleUpdatedEventType =
  | 'node_added'
  | 'node_started'
  | 'node_succeeded'
  | 'node_failed'
  | 'workflow_updated';
export type AgentEventType = WorkflowEventType | 'connected';

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
  visibility?: 'default' | 'recalled' | string;
  retention_ttl_seconds?: number;
  size?: number;
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

// Task & session API types
export interface CreateTaskRequest {
  task: string;
  session_id?: string;
  parent_task_id?: string;
  attachments?: AttachmentUpload[];
}

export interface CreateTaskResponse {
  task_id: string;
  session_id: string;
  parent_task_id?: string | null;
  status?: string;
}

export interface TaskStatusResponse {
  task_id: string;
  session_id?: string;
  parent_task_id?: string | null;
  status: string;
  created_at?: string;
  completed_at?: string | null;
  updated_at?: string;
  final_answer?: string;
  error?: string;
}

export interface Session {
  id: string;
  title?: string | null;
  created_at: string;
  updated_at: string;
  task_count: number;
  last_task?: string | null;
}

export interface SessionTaskSummary {
  task_id: string;
  parent_task_id?: string | null;
  status: string;
  created_at: string;
  updated_at?: string;
  final_answer?: string | null;
}

export interface SessionListResponse {
  sessions: Session[];
}

export interface SessionDetailsResponse {
  session: Session;
  tasks: SessionTaskSummary[];
}

// Evaluation API types
export interface StartEvaluationRequest {
  dataset_path?: string;
  instance_limit?: number;
  max_workers?: number;
  timeout_seconds?: number;
  output_dir?: string;
  report_format?: string;
  enable_metrics?: boolean;
  agent_id?: string;
}

export interface EvaluationMetrics {
  performance: {
    success_rate: number;
    avg_execution_time: number;
    median_time: number;
    p95_time: number;
    timeout_rate: number;
    retry_rate: number;
  };
  quality: {
    solution_quality: number;
    error_recovery_rate: number;
    consistency_score: number;
    complexity_handling: number;
  };
  resources: {
    avg_tokens_used: number;
    total_tokens: number;
    avg_cost_per_task: number;
    total_cost: number;
    memory_usage_mb: number;
  };
  behavior: {
    avg_tool_calls: number;
    tool_usage_pattern: Record<string, number>;
    common_failures: Record<string, number>;
    error_patterns: string[];
  };
  timestamp?: string;
  total_tasks?: number;
  evaluation_id?: string;
}

export interface EvaluationAnalysisSummary {
  overall_score: number;
  performance_grade: string;
  key_strengths?: string[];
  key_weaknesses?: string[];
  risk_level?: string;
}

export interface EvaluationInsight {
  type?: string;
  title: string;
  description: string;
  impact?: string;
  confidence?: number;
}

export interface EvaluationRecommendation {
  title: string;
  description: string;
  priority?: string;
  action_items?: string[];
  expected_improvement?: string;
}

export interface EvaluationTrend {
  performance_trend?: string;
  quality_trend?: string;
  efficiency_trend?: string;
  predicted_score?: number;
  confidence_level?: number;
}

export interface EvaluationAlert {
  level?: string;
  title: string;
  description?: string;
  suggested_action?: string;
  timestamp?: string;
}

export interface EvaluationAnalysis {
  summary: EvaluationAnalysisSummary;
  insights?: EvaluationInsight[];
  recommendations?: EvaluationRecommendation[];
  trends?: EvaluationTrend;
  alerts?: EvaluationAlert[];
  timestamp?: string;
}

export interface EvaluationJobSummary {
  id: string;
  status: string;
  agent_id?: string;
  dataset_path?: string;
  instance_limit?: number;
  max_workers?: number;
  timeout_seconds?: number;
  started_at?: string;
  completed_at?: string;
  summary?: EvaluationAnalysisSummary;
  metrics?: EvaluationMetrics;
  agent?: AgentProfile;
}

export interface EvaluationWorkerResultSummary {
  task_id: string;
  instance_id: string;
  status: string;
  duration_seconds?: number;
  tokens_used?: number;
  cost?: number;
  auto_score?: number;
  grade?: string;
  error?: string;
  files_changed?: number;
  tool_traces?: number;
  started_at?: string;
  completed_at?: string;
}

export interface EvaluationDetailResponse {
  evaluation: EvaluationJobSummary;
  analysis?: EvaluationAnalysis;
  agent?: AgentProfile;
  results?: EvaluationWorkerResultSummary[];
}

export interface AgentProfile {
  agent_id: string;
  config_hash?: string;
  created_at?: string;
  updated_at?: string;
  avg_success_rate?: number;
  avg_exec_time?: number;
  avg_cost_per_task?: number;
  avg_quality_score?: number;
  preferred_tools?: Record<string, number>;
  common_errors?: Record<string, number>;
  strengths?: string[];
  weaknesses?: string[];
  evaluation_count?: number;
  last_evaluated?: string;
  tags?: string[];
  description?: string;
  metadata?: Record<string, any>;
}

export interface EvaluationListResponse {
  evaluations: EvaluationJobSummary[];
}

export type ConfigReadinessSeverity = 'critical' | 'warning' | 'info';

export interface ConfigReadinessTask {
  id: string;
  label: string;
  hint?: string;
  severity?: ConfigReadinessSeverity;
}

export type RuntimeConfigOverrides = Partial<{
  llm_provider: string;
  llm_model: string;
  base_url: string;
  api_key: string;
  ark_api_key: string;
  tavily_api_key: string;
  seedream_text_endpoint_id: string;
  seedream_image_endpoint_id: string;
  seedream_text_model: string;
  seedream_image_model: string;
  seedream_vision_model: string;
  seedream_video_model: string;
  environment: string;
  agent_preset: string;
  tool_preset: string;
  session_dir: string;
  cost_dir: string;
  max_tokens: number;
  max_iterations: number;
  temperature: number;
  top_p: number;
  stop_sequences: string[];
  verbose: boolean;
  disable_tui: boolean;
  follow_transcript: boolean;
  follow_stream: boolean;
}>;

export interface RuntimeConfigOverridesPayload {
  overrides: RuntimeConfigOverrides;
}

export interface RuntimeConfigSnapshot {
  effective?: RuntimeConfigOverrides;
  overrides?: RuntimeConfigOverrides;
  readiness?: ConfigReadinessTask[];
  sources?: Record<string, string>;
  tasks?: ConfigReadinessTask[];
  updated_at?: string;
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
  attachments?: Record<string, AttachmentPayload> | null;
}

export interface Message {
  role: string;
  content: string;
  tool_calls?: ToolCall[];
  tool_results?: ToolResult[];
  tool_call_id?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload> | null;
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

// Backwards-compatible alias for historical base event interface
export type AgentEvent<TPayload = Record<string, unknown>> = WorkflowEnvelope<AgentEventType> & TPayload;

export interface WorkflowLifecycleUpdatedPayload {
  workflow_id?: string;
  workflow_event_type: WorkflowLifecycleUpdatedEventType;
  phase?: WorkflowPhase;
  node?: WorkflowNodeSnapshot;
  workflow?: WorkflowSnapshot;
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
  attachments?: Record<string, AttachmentPayload> | null;
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
  attachments?: Record<string, AttachmentPayload> | null;
}

export interface WorkflowResultFinalPayload {
  final_answer: string;
  total_iterations: number;
  total_tokens: number;
  stop_reason: string;
  duration: number;
  is_streaming?: boolean;
  stream_finished?: boolean;
  attachments?: Record<string, AttachmentPayload> | null;
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

export interface WorkflowDiagnosticEnvironmentSnapshotPayload {
  host?: Record<string, string> | null;
  captured: string;
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
  attachments?: Record<string, AttachmentPayload> | null;
}

export type WorkflowLifecycleUpdatedEvent = WorkflowEvent<
  WorkflowLifecycleUpdatedPayload,
  'workflow.lifecycle.updated'
>;
export type WorkflowNodeStartedEvent = WorkflowEvent<
  WorkflowNodeStartedPayload,
  'workflow.node.started'
>;
export type WorkflowNodeCompletedEvent = WorkflowEvent<
  WorkflowNodeCompletedPayload,
  'workflow.node.completed'
>;
export type WorkflowNodeFailedEvent = WorkflowEvent<
  WorkflowNodeFailedPayload,
  'workflow.node.failed'
>;
export type WorkflowNodeOutputDeltaEvent = WorkflowEvent<
  WorkflowNodeOutputDeltaPayload,
  'workflow.node.output.delta'
>;
export type WorkflowNodeOutputSummaryEvent = WorkflowEvent<
  WorkflowNodeOutputSummaryPayload,
  'workflow.node.output.summary'
>;
export type WorkflowToolStartedEvent = WorkflowEvent<
  WorkflowToolStartedPayload,
  'workflow.tool.started'
>;
export type WorkflowToolProgressEvent = WorkflowEvent<
  WorkflowToolProgressPayload,
  'workflow.tool.progress'
>;
export type WorkflowToolCompletedEvent = WorkflowEvent<
  WorkflowToolCompletedPayload,
  'workflow.tool.completed'
>;
export type WorkflowResultFinalEvent = WorkflowEvent<
  WorkflowResultFinalPayload,
  'workflow.result.final'
>;
export type WorkflowResultCancelledEvent = WorkflowEvent<
  WorkflowResultCancelledPayload,
  'workflow.result.cancelled'
>;
export type WorkflowSubflowProgressEvent = WorkflowEvent<
  WorkflowSubflowProgressPayload,
  'workflow.subflow.progress'
>;
export type WorkflowSubflowCompletedEvent = WorkflowEvent<
  WorkflowSubflowCompletedPayload,
  'workflow.subflow.completed'
>;
export type WorkflowDiagnosticEnvironmentSnapshotEvent = WorkflowEvent<
  WorkflowDiagnosticEnvironmentSnapshotPayload,
  'workflow.diagnostic.environment_snapshot'
>;
export type WorkflowDiagnosticContextCompressionEvent = WorkflowEvent<
  WorkflowDiagnosticContextCompressionPayload,
  'workflow.diagnostic.context_compression'
>;
export type WorkflowDiagnosticToolFilteringEvent = WorkflowEvent<
  WorkflowDiagnosticToolFilteringPayload,
  'workflow.diagnostic.tool_filtering'
>;
export type WorkflowDiagnosticContextSnapshotEvent = WorkflowEvent<
  WorkflowDiagnosticContextSnapshotPayload,
  'workflow.diagnostic.context_snapshot'
>;
export interface WorkflowDiagnosticErrorPayload {
  iteration?: number;
  phase?: string;
  recoverable?: boolean;
  error?: string;
}

export type WorkflowDiagnosticErrorEvent = WorkflowEvent<WorkflowDiagnosticErrorPayload, 'workflow.diagnostic.error'>;
export type WorkflowInputReceivedEvent = WorkflowEvent<UserTaskPayload, 'workflow.input.received'>;

export interface ConnectedEvent {
  event_type: 'connected';
  session_id: string;
  task_id?: string;
  parent_task_id?: string;
  timestamp?: string;
  agent_level?: AgentLevel;
}

export type AnyAgentEvent =
  | WorkflowLifecycleUpdatedEvent
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
  | WorkflowDiagnosticEnvironmentSnapshotEvent
  | WorkflowDiagnosticContextCompressionEvent
  | WorkflowDiagnosticToolFilteringEvent
  | WorkflowDiagnosticContextSnapshotEvent
  | WorkflowDiagnosticErrorEvent
  | WorkflowInputReceivedEvent
  | ConnectedEvent;

export function canonicalEventType(type: AgentEventType | string): AgentEventType {
  return type as AgentEventType;
}

export function eventMatches(
  event: AnyAgentEvent,
  ...types: (WorkflowEventType | 'connected')[]
): boolean {
  const canonicalEvent = canonicalEventType(event.event_type);
  return types.some((type) => canonicalEventType(type) === canonicalEvent);
}
