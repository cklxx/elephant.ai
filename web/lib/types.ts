// TypeScript types for ALEX Web Frontend
// Corresponds to Go event types in internal/agent/domain/events.go

export type AgentLevel = 'core' | 'subagent';

// Base interface for all agent events
export interface AgentEvent {
  event_type: string;
  timestamp: string;
  agent_level: AgentLevel;
  session_id: string;
  task_id?: string;
  parent_task_id?: string;
}

// Task Analysis Event - emitted after task pre-analysis
export interface TaskAnalysisEvent extends AgentEvent {
  event_type: 'task_analysis';
  action_name: string; // e.g., "Optimizing context collection pipeline"
  goal: string; // Brief description of what needs to be achieved
}

// Iteration Start Event - emitted at start of each ReAct iteration
export interface IterationStartEvent extends AgentEvent {
  event_type: 'iteration_start';
  iteration: number;
  total_iters: number;
}

// Thinking Event - emitted when LLM is generating response
export interface ThinkingEvent extends AgentEvent {
  event_type: 'thinking';
  iteration: number;
  message_count: number;
}

// Think Complete Event - emitted when LLM response received
export interface ThinkCompleteEvent extends AgentEvent {
  event_type: 'think_complete';
  iteration: number;
  content: string;
  tool_call_count: number;
}

// Tool Call Start Event - emitted when tool execution begins
export interface ToolCallStartEvent extends AgentEvent {
  event_type: 'tool_call_start';
  iteration: number;
  call_id: string;
  tool_name: string;
  arguments: Record<string, any>;
  arguments_preview?: string;
}

// Tool Call Stream Event - emitted during tool execution (for streaming tools)
export interface ToolCallStreamEvent extends AgentEvent {
  event_type: 'tool_call_stream';
  call_id: string;
  chunk: string;
  is_complete: boolean;
}

export interface AttachmentPayload {
  name: string;
  media_type: string;
  data?: string;
  uri?: string;
  source?: string;
  description?: string;
}

export interface AttachmentUpload {
  name: string;
  media_type: string;
  data?: string;
  uri?: string;
  source?: string;
  description?: string;
}

export type MessageSource =
  | 'system_prompt'
  | 'user_input'
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

// Tool Call Complete Event - emitted when tool execution finishes
export interface ToolCallCompleteEvent extends AgentEvent {
  event_type: 'tool_call_complete';
  call_id: string;
  tool_name: string;
  result: string;
  error?: string;
  duration: number; // milliseconds
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
}

// Iteration Complete Event - emitted at end of iteration
export interface IterationCompleteEvent extends AgentEvent {
  event_type: 'iteration_complete';
  iteration: number;
  tokens_used: number;
  tools_run: number;
}

// Task Complete Event - emitted when entire task finishes
export interface TaskCompleteEvent extends AgentEvent {
  event_type: 'task_complete';
  final_answer: string;
  total_iterations: number;
  total_tokens: number;
  stop_reason: string;
  duration: number; // milliseconds
  attachments?: Record<string, AttachmentPayload>;
}

// Task Cancelled Event - emitted when a task receives a cancellation request
export interface TaskCancelledEvent extends AgentEvent {
  event_type: 'task_cancelled';
  reason: string;
  requested_by?: 'user' | 'system';
}

// Error Event - emitted on errors
export interface ErrorEvent extends AgentEvent {
  event_type: 'error';
  iteration: number;
  phase: string; // "think", "execute", "observe"
  error: string;
  recoverable: boolean;
}

// Research Plan Event - emitted when agent creates a research/execution plan
export interface ResearchPlanEvent extends AgentEvent {
  event_type: 'research_plan';
  plan_steps: string[];
  estimated_iterations: number;
  estimated_tools?: string[];
  estimated_duration_minutes?: number;
}

// Step Started Event - emitted when a research step begins
export interface StepStartedEvent extends AgentEvent {
  event_type: 'step_started';
  step_index: number;
  step_description: string;
  iteration?: number;
}

// Step Completed Event - emitted when a research step finishes
export interface StepCompletedEvent extends AgentEvent {
  event_type: 'step_completed';
  step_index: number;
  step_result: string;
  iteration?: number;
  step_description?: string;
}

// Browser Info Event - emitted when sandbox browser diagnostics are captured
export interface BrowserInfoEvent extends AgentEvent {
  event_type: 'browser_info';
  success?: boolean;
  message?: string;
  user_agent?: string;
  cdp_url?: string;
  vnc_url?: string;
  viewport_width?: number;
  viewport_height?: number;
  captured: string;
}

// Environment Snapshot Event - emitted when host/sandbox environments are captured
export interface EnvironmentSnapshotEvent extends AgentEvent {
  event_type: 'environment_snapshot';
  host?: Record<string, string> | null;
  sandbox?: Record<string, string> | null;
  captured: string;
}

export type SandboxProgressStatus = 'pending' | 'running' | 'ready' | 'error';

// Sandbox Progress Event - emitted during sandbox initialization lifecycle
export interface SandboxProgressEvent extends AgentEvent {
  event_type: 'sandbox_progress';
  status: SandboxProgressStatus;
  stage: string;
  message?: string;
  step: number;
  total_steps: number;
  error?: string;
  updated: string;
}

// Context Compression Event - emitted when context is compressed
export interface ContextCompressionEvent extends AgentEvent {
  event_type: 'context_compression';
  original_count: number;
  compressed_count: number;
  compression_rate: number;
}

// Tool Filtering Event - emitted when tools are filtered by preset
export interface ToolFilteringEvent extends AgentEvent {
  event_type: 'tool_filtering';
  preset_name: string;
  original_count: number;
  filtered_count: number;
  filtered_tools: string[];
  tool_filter_ratio: number;
}

export interface ContextSnapshotEvent extends AgentEvent {
  event_type: 'context_snapshot';
  iteration: number;
  request_id: string;
  messages: Message[];
  excluded_messages?: Message[];
}

// Connected Event - emitted when SSE connection is established
export interface ConnectedEvent {
  event_type: 'connected';
  session_id: string;
  task_id?: string;
  parent_task_id?: string;
  timestamp?: string;
  agent_level?: AgentLevel;
}

// User Task Event - client-side only event to display user's task
export interface UserTaskEvent extends AgentEvent {
  event_type: 'user_task';
  task: string;
  attachments?: Record<string, AttachmentPayload>;
}

// Union type for all agent events
export type AnyAgentEvent =
  | TaskAnalysisEvent
  | IterationStartEvent
  | ThinkingEvent
  | ThinkCompleteEvent
  | ToolCallStartEvent
  | ToolCallStreamEvent
  | ToolCallCompleteEvent
  | IterationCompleteEvent
  | TaskCancelledEvent
  | TaskCompleteEvent
  | ErrorEvent
  | ResearchPlanEvent
  | StepStartedEvent
  | StepCompletedEvent
  | BrowserInfoEvent
  | EnvironmentSnapshotEvent
  | SandboxProgressEvent
  | ContextCompressionEvent
  | ToolFilteringEvent
  | ContextSnapshotEvent
  | ConnectedEvent
  | UserTaskEvent;

// API Request/Response Types

export interface CreateTaskRequest {
  task: string;
  session_id?: string;
  agent_preset?: string; // Agent persona preset (e.g., "default", "code-expert")
  tool_preset?: string;  // Tool access preset (e.g., "full", "read-only")
  attachments?: AttachmentUpload[];
}

export interface CreateTaskResponse {
  task_id: string;
  session_id: string;
  parent_task_id?: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  requires_plan_approval?: boolean; // If true, wait for plan approval before execution
}

export interface ResearchPlan {
  goal: string;
  steps: string[];
  estimated_tools: string[];
  estimated_iterations: number;
  estimated_duration_minutes?: number;
}

export interface ApprovePlanRequest {
  session_id: string;
  task_id: string;
  approved: boolean;
  modified_plan?: ResearchPlan;
  rejection_reason?: string;
}

export interface ApprovePlanResponse {
  success: boolean;
  message: string;
}

export interface TaskStatusResponse {
  task_id: string;
  session_id: string;
  parent_task_id?: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  created_at: string;
  completed_at?: string;
  error?: string;
}

export interface Session {
  id: string;
  created_at: string;
  updated_at: string;
  task_count: number;
  last_task?: string;
}

export interface SessionListResponse {
  sessions: Session[];
  total: number;
}

export interface SessionDetailsResponse {
  session: Session;
  tasks: TaskStatusResponse[];
}

// UI State Types

export interface ConnectionStatus {
  connected: boolean;
  reconnecting: boolean;
  error?: string;
}

export interface ToolCategory {
  file: string;
  shell: string;
  search: string;
  web: string;
  think: string;
  task: string;
}
