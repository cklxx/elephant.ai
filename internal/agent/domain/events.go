package domain

import (
	"time"

	"alex/internal/agent/ports"
	"alex/internal/workflow"
)

// Re-export the event listener contract defined at the port layer.
type AgentEvent = ports.AgentEvent
type EventListener = ports.EventListener

// BaseEvent provides common fields for all events
type BaseEvent struct {
	timestamp    time.Time
	agentLevel   ports.AgentLevel
	sessionID    string
	taskID       string
	parentTaskID string
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

func (e *BaseEvent) GetAgentLevel() ports.AgentLevel {
	return e.agentLevel
}

func (e *BaseEvent) GetSessionID() string {
	return e.sessionID
}

func (e *BaseEvent) GetTaskID() string {
	return e.taskID
}

func (e *BaseEvent) GetParentTaskID() string {
	return e.parentTaskID
}

func newBaseEventWithIDs(level ports.AgentLevel, sessionID, taskID, parentTaskID string, ts time.Time) BaseEvent {
	return BaseEvent{
		timestamp:    ts,
		agentLevel:   level,
		sessionID:    sessionID,
		taskID:       taskID,
		parentTaskID: parentTaskID,
	}
}

// NewBaseEvent exposes construction of BaseEvent for adapters that need to bridge
// external lifecycle systems (e.g., workflows) into the agent event stream while
// preserving field encapsulation.
func NewBaseEvent(level ports.AgentLevel, sessionID, taskID, parentTaskID string, ts time.Time) BaseEvent {
	return newBaseEventWithIDs(level, sessionID, taskID, parentTaskID, ts)
}

// WorkflowInputReceivedEvent - emitted when a user submits a new task
type WorkflowInputReceivedEvent struct {
	BaseEvent
	Task        string
	Attachments map[string]ports.Attachment
}

func (e *WorkflowInputReceivedEvent) EventType() string { return "workflow.input.received" }

// WorkflowPlanCreatedEvent is emitted once the planner has produced the ordered
// list of steps that will be executed.
type WorkflowPlanCreatedEvent struct {
	BaseEvent
	Steps []string
}

func (e *WorkflowPlanCreatedEvent) EventType() string { return "workflow.plan.created" }

// NewWorkflowInputReceivedEvent constructs a user task event with the provided metadata.
func NewWorkflowInputReceivedEvent(
	level ports.AgentLevel,
	sessionID, taskID, parentTaskID string,
	task string,
	attachments map[string]ports.Attachment,
	ts time.Time,
) *WorkflowInputReceivedEvent {
	var cloned map[string]ports.Attachment
	if len(attachments) > 0 {
		cloned = ports.CloneAttachmentMap(attachments)
	}

	return &WorkflowInputReceivedEvent{
		BaseEvent:   newBaseEventWithIDs(level, sessionID, taskID, parentTaskID, ts),
		Task:        task,
		Attachments: cloned,
	}
}

// WorkflowNodeStartedEvent - emitted at start of a workflow node (iteration or step)
type WorkflowNodeStartedEvent struct {
	BaseEvent
	Iteration       int
	TotalIters      int
	StepIndex       int
	StepDescription string
	Input           any
	Workflow        *workflow.WorkflowSnapshot
}

func (e *WorkflowNodeStartedEvent) EventType() string { return "workflow.node.started" }

// WorkflowNodeOutputDeltaEvent - emitted when LLM is generating response or streaming content
type WorkflowNodeOutputDeltaEvent struct {
	BaseEvent
	Iteration    int
	MessageCount int
	Delta        string
	Final        bool
	CreatedAt    time.Time
	SourceModel  string
}

func (e *WorkflowNodeOutputDeltaEvent) EventType() string { return "workflow.node.output.delta" }

// WorkflowNodeOutputSummaryEvent - emitted when an LLM response finishes
type WorkflowNodeOutputSummaryEvent struct {
	BaseEvent
	Iteration     int
	Content       string
	ToolCallCount int
}

func (e *WorkflowNodeOutputSummaryEvent) EventType() string { return "workflow.node.output.summary" }

// WorkflowLifecycleUpdatedEvent mirrors raw workflow transitions so consumers can render
// timeline updates without inferring state from step events alone.
type WorkflowLifecycleUpdatedEvent struct {
	BaseEvent
	WorkflowID        string
	WorkflowEventType workflow.EventType
	Phase             workflow.WorkflowPhase
	Node              *workflow.NodeSnapshot
	Workflow          *workflow.WorkflowSnapshot
}

func (e *WorkflowLifecycleUpdatedEvent) EventType() string { return "workflow.lifecycle.updated" }

// WorkflowNodeCompletedEvent - emitted when a workflow node finishes (step or iteration)
type WorkflowNodeCompletedEvent struct {
	BaseEvent
	StepIndex       int
	StepDescription string
	StepResult      any
	Status          string
	Iteration       int
	TokensUsed      int
	ToolsRun        int
	Workflow        *workflow.WorkflowSnapshot
}

func (e *WorkflowNodeCompletedEvent) EventType() string { return "workflow.node.completed" }

// WorkflowToolStartedEvent - emitted when tool execution begins
type WorkflowToolStartedEvent struct {
	BaseEvent
	Iteration int
	CallID    string
	ToolName  string
	Arguments map[string]interface{}
}

func (e *WorkflowToolStartedEvent) EventType() string { return "workflow.tool.started" }

// WorkflowToolProgressEvent - emitted during tool execution (for streaming tools)
type WorkflowToolProgressEvent struct {
	BaseEvent
	CallID     string
	Chunk      string
	IsComplete bool
}

func (e *WorkflowToolProgressEvent) EventType() string { return "workflow.tool.progress" }

// WorkflowToolCompletedEvent - emitted when tool execution finishes
type WorkflowToolCompletedEvent struct {
	BaseEvent
	CallID      string
	ToolName    string
	Result      string
	Error       error
	Duration    time.Duration
	Metadata    map[string]any
	Attachments map[string]ports.Attachment
}

func (e *WorkflowToolCompletedEvent) EventType() string { return "workflow.tool.completed" }

// WorkflowResultFinalEvent - emitted when entire task finishes
type WorkflowResultFinalEvent struct {
	BaseEvent
	FinalAnswer     string
	TotalIterations int
	TotalTokens     int
	StopReason      string
	Duration        time.Duration
	// IsStreaming signals that this event represents an in-flight streaming
	// update of the final answer rather than the terminal completion.
	IsStreaming bool
	// StreamFinished marks that the streaming sequence has delivered its
	// final payload. Non-streamed completions set this to true by default.
	StreamFinished bool
	SessionStats   *ports.SessionStats // Optional: session-level cost/token accumulation
	Attachments    map[string]ports.Attachment
}

func (e *WorkflowResultFinalEvent) EventType() string { return "workflow.result.final" }

// WorkflowResultCancelledEvent - emitted when a running task receives an explicit cancellation request
type WorkflowResultCancelledEvent struct {
	BaseEvent
	Reason      string
	RequestedBy string
}

func (e *WorkflowResultCancelledEvent) EventType() string { return "workflow.result.cancelled" }

// NewWorkflowResultCancelledEvent constructs a cancellation notification event for SSE consumers.
func NewWorkflowResultCancelledEvent(
	level ports.AgentLevel,
	sessionID, taskID, parentTaskID string,
	reason, requestedBy string,
	ts time.Time,
) *WorkflowResultCancelledEvent {
	return &WorkflowResultCancelledEvent{
		BaseEvent:   newBaseEventWithIDs(level, sessionID, taskID, parentTaskID, ts),
		Reason:      reason,
		RequestedBy: requestedBy,
	}
}

// WorkflowNodeFailedEvent - emitted on errors
type WorkflowNodeFailedEvent struct {
	BaseEvent
	Iteration   int
	Phase       string // "think", "execute", "observe"
	Error       error
	Recoverable bool
}

func (e *WorkflowNodeFailedEvent) EventType() string { return "workflow.node.failed" }

// WorkflowDiagnosticContextCompressionEvent - emitted when context is compressed
type WorkflowDiagnosticContextCompressionEvent struct {
	BaseEvent
	OriginalCount   int
	CompressedCount int
	CompressionRate float64 // percentage of messages retained
}

func (e *WorkflowDiagnosticContextCompressionEvent) EventType() string {
	return "workflow.diagnostic.context_compression"
}

// NewWorkflowDiagnosticContextCompressionEvent creates a new context compression event
func NewWorkflowDiagnosticContextCompressionEvent(level ports.AgentLevel, sessionID, taskID, parentTaskID string, originalCount, compressedCount int, ts time.Time) *WorkflowDiagnosticContextCompressionEvent {
	compressionRate := 0.0
	if originalCount > 0 {
		compressionRate = float64(compressedCount) / float64(originalCount) * 100.0
	}
	return &WorkflowDiagnosticContextCompressionEvent{
		BaseEvent:       newBaseEventWithIDs(level, sessionID, taskID, parentTaskID, ts),
		OriginalCount:   originalCount,
		CompressedCount: compressedCount,
		CompressionRate: compressionRate,
	}
}

// WorkflowDiagnosticContextSnapshotEvent - emitted with the exact messages provided to the LLM.
type WorkflowDiagnosticContextSnapshotEvent struct {
	BaseEvent
	Iteration  int
	LLMTurnSeq int
	RequestID  string
	Messages   []ports.Message
	Excluded   []ports.Message
}

func (e *WorkflowDiagnosticContextSnapshotEvent) EventType() string {
	return "workflow.diagnostic.context_snapshot"
}

// NewWorkflowDiagnosticContextSnapshotEvent creates an immutable snapshot of the LLM context payload.
func NewWorkflowDiagnosticContextSnapshotEvent(
	level ports.AgentLevel,
	sessionID, taskID, parentTaskID string,
	iteration int,
	llmTurnSeq int,
	requestID string,
	messages, excluded []ports.Message,
	ts time.Time,
) *WorkflowDiagnosticContextSnapshotEvent {
	return &WorkflowDiagnosticContextSnapshotEvent{
		BaseEvent:  newBaseEventWithIDs(level, sessionID, taskID, parentTaskID, ts),
		Iteration:  iteration,
		LLMTurnSeq: llmTurnSeq,
		RequestID:  requestID,
		Messages:   cloneMessageSlice(messages),
		Excluded:   cloneMessageSlice(excluded),
	}
}

// WorkflowDiagnosticToolFilteringEvent - emitted when tools are filtered by preset
type WorkflowDiagnosticToolFilteringEvent struct {
	BaseEvent
	PresetName      string
	OriginalCount   int
	FilteredCount   int
	FilteredTools   []string
	ToolFilterRatio float64 // percentage of tools retained
}

func (e *WorkflowDiagnosticToolFilteringEvent) EventType() string {
	return "workflow.diagnostic.tool_filtering"
}

// NewWorkflowDiagnosticToolFilteringEvent creates a new tool filtering event
func NewWorkflowDiagnosticToolFilteringEvent(level ports.AgentLevel, sessionID, taskID, parentTaskID, presetName string, originalCount, filteredCount int, filteredTools []string, ts time.Time) *WorkflowDiagnosticToolFilteringEvent {
	filterRatio := 0.0
	if originalCount > 0 {
		filterRatio = float64(filteredCount) / float64(originalCount) * 100.0
	}
	return &WorkflowDiagnosticToolFilteringEvent{
		BaseEvent:       newBaseEventWithIDs(level, sessionID, taskID, parentTaskID, ts),
		PresetName:      presetName,
		OriginalCount:   originalCount,
		FilteredCount:   filteredCount,
		FilteredTools:   filteredTools,
		ToolFilterRatio: filterRatio,
	}
}

// WorkflowDiagnosticEnvironmentSnapshotEvent - emitted when host environment is captured
type WorkflowDiagnosticEnvironmentSnapshotEvent struct {
	BaseEvent
	Host     map[string]string
	Captured time.Time
}

func (e *WorkflowDiagnosticEnvironmentSnapshotEvent) EventType() string {
	return "workflow.diagnostic.environment_snapshot"
}

// NewWorkflowDiagnosticEnvironmentSnapshotEvent constructs a new environment snapshot event.
func NewWorkflowDiagnosticEnvironmentSnapshotEvent(host map[string]string, captured time.Time) *WorkflowDiagnosticEnvironmentSnapshotEvent {
	return &WorkflowDiagnosticEnvironmentSnapshotEvent{
		BaseEvent: newBaseEventWithIDs(ports.LevelCore, "", "", "", captured),
		Host:      cloneStringMap(host),
		Captured:  captured,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for k, v := range values {
		clone[k] = v
	}
	return clone
}

func cloneMessageSlice(values []ports.Message) []ports.Message {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]ports.Message, len(values))
	for i, msg := range values {
		cloned[i] = cloneMessage(msg)
	}
	return cloned
}

func cloneMessage(msg ports.Message) ports.Message {
	cloned := msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ports.ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneToolResult(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(msg.Attachments)
	}
	return cloned
}

func cloneToolResult(result ports.ToolResult) ports.ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(result.Attachments)
	}
	return cloned
}

// EventListenerFunc is a function adapter for EventListener
type EventListenerFunc func(AgentEvent)

func (f EventListenerFunc) OnEvent(event AgentEvent) {
	f(event)
}
