// TypeScript types for ALEX Web Frontend
// Corresponds to Go event types in internal/agent/domain/events.go

export type AgentLevel = 'core' | 'subagent';

// Base interface for all agent events
export interface AgentEvent {
  event_type: string;
  timestamp: string;
  agent_level: AgentLevel;
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
}

// Tool Call Stream Event - emitted during tool execution (for streaming tools)
export interface ToolCallStreamEvent extends AgentEvent {
  event_type: 'tool_call_stream';
  call_id: string;
  chunk: string;
  is_complete: boolean;
}

// Tool Call Complete Event - emitted when tool execution finishes
export interface ToolCallCompleteEvent extends AgentEvent {
  event_type: 'tool_call_complete';
  call_id: string;
  tool_name: string;
  result: string;
  error?: string;
  duration: number; // milliseconds
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

// Browser Snapshot Event - emitted when capturing browser state (for web automation tools)
export interface BrowserSnapshotEvent extends AgentEvent {
  event_type: 'browser_snapshot';
  url: string;
  screenshot_data?: string; // base64 encoded image
  html_preview?: string; // truncated HTML for preview
}

// User Task Event - client-side only event to display user's task
export interface UserTaskEvent extends AgentEvent {
  event_type: 'user_task';
  task: string;
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
  | TaskCompleteEvent
  | ErrorEvent
  | ResearchPlanEvent
  | StepStartedEvent
  | StepCompletedEvent
  | BrowserSnapshotEvent
  | UserTaskEvent;

// API Request/Response Types

export interface CreateTaskRequest {
  task: string;
  session_id?: string;
  auto_approve_plan?: boolean; // If true, skip plan approval flow
}

export interface CreateTaskResponse {
  task_id: string;
  session_id: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  requires_plan_approval?: boolean; // If true, wait for plan approval before execution
}

export interface ResearchPlan {
  goal: string;
  steps: string[];
  estimated_tools: string[];
  estimated_iterations: number;
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
  status: 'pending' | 'running' | 'completed' | 'failed';
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
