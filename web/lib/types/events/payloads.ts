import type { AttachmentPayload } from '../ui/attachment';
import type { Message } from '../api/context';
import type {
  WorkflowLifecycleUpdatedEventType,
  WorkflowNodeSnapshot,
  WorkflowSnapshot,
  WorkflowPhase,
  WorkflowNodeStatus,
} from './base';

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

export interface ToolSLAPayload {
  tool_name: string;
  p50_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
  error_rate: number;
  call_count: number;
  success_rate: number;
  cost_usd_total: number;
  cost_usd_avg: number;
}

export interface WorkflowToolCompletedPayload {
  call_id: string;
  tool_name: string;
  result: unknown;
  error?: string;
  duration: number;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload> | null;
  tool_sla?: ToolSLAPayload;
}

export interface WorkflowReplanRequestedPayload {
  call_id?: string;
  tool_name?: string;
  reason?: string;
  error?: string;
}

export interface WorkflowArtifactManifestPayload {
  manifest?: any;
  attachments?: Record<string, AttachmentPayload> | null;
  source_tool?: string;
  result?: string;
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

export interface WorkflowDiagnosticErrorPayload {
  iteration?: number;
  phase?: string;
  recoverable?: boolean;
  error?: string;
}

export interface UserTaskPayload {
  task: string;
  attachments?: Record<string, AttachmentPayload> | null;
}
