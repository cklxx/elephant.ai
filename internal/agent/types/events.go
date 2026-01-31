package types

// Event type constants — single source of truth for all workflow event names.
// Referenced by agent domain, server app, SSE handler, and translators.

const (
	// Core workflow lifecycle
	EventInputReceived    = "workflow.input.received"
	EventLifecycleUpdated = "workflow.lifecycle.updated"

	// Node lifecycle
	EventNodeStarted       = "workflow.node.started"
	EventNodeCompleted     = "workflow.node.completed"
	EventNodeFailed        = "workflow.node.failed"
	EventNodeOutputDelta   = "workflow.node.output.delta"
	EventNodeOutputSummary = "workflow.node.output.summary"

	// Tool lifecycle
	EventToolStarted   = "workflow.tool.started"
	EventToolProgress  = "workflow.tool.progress"
	EventToolCompleted = "workflow.tool.completed"

	// Subflow (subtask delegation)
	EventSubflowProgress  = "workflow.subflow.progress"
	EventSubflowCompleted = "workflow.subflow.completed"

	// Result terminal events
	EventResultFinal     = "workflow.result.final"
	EventResultCancelled = "workflow.result.cancelled"

	// Diagnostics
	EventDiagnosticError              = "workflow.diagnostic.error"
	EventDiagnosticPreanalysisEmoji   = "workflow.diagnostic.preanalysis_emoji"
	EventDiagnosticContextCompression = "workflow.diagnostic.context_compression"
	EventDiagnosticContextSnapshot    = "workflow.diagnostic.context_snapshot"
	EventDiagnosticEnvironmentSnapshot = "workflow.diagnostic.environment_snapshot"
	EventDiagnosticToolFiltering      = "workflow.diagnostic.tool_filtering"

	// Artifact
	EventArtifactManifest = "workflow.artifact.manifest"

	// Executor (ACP)
	EventExecutorUpdate      = "workflow.executor.update"
	EventExecutorUserMessage = "workflow.executor.user_message"

	// Proactive
	EventProactiveContextRefresh = "proactive.context.refresh"

	// Background tasks
	EventBackgroundTaskDispatched = "background.task.dispatched"
	EventBackgroundTaskCompleted  = "background.task.completed"

	// Stream infrastructure (synthesized by EventBroadcaster, not by agent)
	EventStreamDropped = "workflow.stream.dropped"
)
