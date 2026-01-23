import type {
  AgentEventType,
  WorkflowEventType,
  WorkflowEvent,
  ConnectedEvent,
} from './base';
import type {
  WorkflowLifecycleUpdatedPayload,
  WorkflowNodeStartedPayload,
  WorkflowNodeCompletedPayload,
  WorkflowNodeFailedPayload,
  WorkflowNodeOutputDeltaPayload,
  WorkflowNodeOutputSummaryPayload,
  WorkflowToolStartedPayload,
  WorkflowToolProgressPayload,
  WorkflowToolCompletedPayload,
  WorkflowArtifactManifestPayload,
  WorkflowResultFinalPayload,
  WorkflowResultCancelledPayload,
  WorkflowSubflowProgressPayload,
  WorkflowSubflowCompletedPayload,
  WorkflowDiagnosticEnvironmentSnapshotPayload,
  WorkflowDiagnosticContextCompressionPayload,
  WorkflowDiagnosticToolFilteringPayload,
  WorkflowDiagnosticContextSnapshotPayload,
  WorkflowDiagnosticErrorPayload,
  UserTaskPayload,
} from './payloads';

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
export type WorkflowArtifactManifestEvent = WorkflowEvent<
  WorkflowArtifactManifestPayload,
  'workflow.artifact.manifest'
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
export type WorkflowDiagnosticErrorEvent = WorkflowEvent<
  WorkflowDiagnosticErrorPayload,
  'workflow.diagnostic.error'
>;
export type WorkflowInputReceivedEvent = WorkflowEvent<
  UserTaskPayload,
  'workflow.input.received'
>;

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
  | WorkflowArtifactManifestEvent
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

export type { AgentEventType, WorkflowEventType };
