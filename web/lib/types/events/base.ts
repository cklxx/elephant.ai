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
  | 'workflow.artifact.manifest'
  | 'workflow.input.received'
  | 'workflow.subflow.progress'
  | 'workflow.subflow.completed'
  | 'workflow.result.final'
  | 'workflow.result.cancelled'
  | 'workflow.diagnostic.error'
  | 'workflow.diagnostic.context_compression'
  | 'workflow.diagnostic.tool_filtering'
  | 'workflow.diagnostic.environment_snapshot'
  | 'workflow.diagnostic.context_snapshot'
  | 'proactive.context.refresh';

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

export interface WorkflowEnvelope<TType extends AgentEventType = WorkflowEventType> {
  // Identity
  event_id?: string;
  version?: number;
  event_type: TType;
  seq?: number;
  timestamp: string;

  // Hierarchy
  agent_level: AgentLevel;
  session_id: string;
  run_id?: string;
  parent_run_id?: string;

  // Causal chain
  correlation_id?: string;
  causation_id?: string;

  // Workflow
  workflow_id?: string;
  node_id?: string;
  node_kind?: string;

  // Subtask metadata
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

export interface ConnectedEvent {
  event_type: 'connected';
  session_id: string;
  run_id?: string;
  parent_run_id?: string;
  active_run_id?: string;
  timestamp?: string;
  agent_level?: AgentLevel;
}
