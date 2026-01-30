package domain

import (
	"sync/atomic"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/ports/storage"
	"alex/internal/utils/id"
	"alex/internal/workflow"
)

// Re-export the event listener contract defined at the port layer.
type AgentEvent = agent.AgentEvent
type EventListener = agent.EventListener

// BaseEvent provides common fields for all events
type BaseEvent struct {
	// Identity
	eventID string // Unique per event: "evt-{ksuid}"
	seq     uint64 // Monotonic within a run

	// Temporal
	timestamp time.Time

	// Hierarchy
	sessionID   string           // Conversation scope
	runID       string           // This agent execution
	parentRunID string           // Parent agent's runID (empty for core)
	agentLevel  agent.AgentLevel // "core" or "subagent"

	// Causal chain
	correlationID string // Root runID of the causal chain
	causationID   string // call_id that spawned this run

	// Operational
	logID string // Log correlation
}

func (e *BaseEvent) Timestamp() time.Time            { return e.timestamp }
func (e *BaseEvent) GetAgentLevel() agent.AgentLevel { return e.agentLevel }
func (e *BaseEvent) GetSessionID() string            { return e.sessionID }
func (e *BaseEvent) GetRunID() string                { return e.runID }
func (e *BaseEvent) GetParentRunID() string          { return e.parentRunID }
func (e *BaseEvent) GetCorrelationID() string        { return e.correlationID }
func (e *BaseEvent) GetCausationID() string          { return e.causationID }
func (e *BaseEvent) GetEventID() string              { return e.eventID }
func (e *BaseEvent) GetSeq() uint64                  { return e.seq }
func (e *BaseEvent) GetLogID() string                { return e.logID }

// SetLogID attaches a log identifier for correlation.
func (e *BaseEvent) SetLogID(logID string) { e.logID = logID }

// SeqCounter provides monotonic event sequence numbering within a run.
type SeqCounter struct {
	counter atomic.Uint64
}

// Next returns the next sequence number.
func (s *SeqCounter) Next() uint64 {
	return s.counter.Add(1)
}

func newBaseEventWithIDs(level agent.AgentLevel, sessionID, runID, parentRunID string, ts time.Time) BaseEvent {
	return BaseEvent{
		eventID:     id.NewEventID(),
		timestamp:   ts,
		agentLevel:  level,
		sessionID:   sessionID,
		runID:       runID,
		parentRunID: parentRunID,
	}
}

// NewBaseEvent exposes construction of BaseEvent for adapters that need to bridge
// external lifecycle systems (e.g., workflows) into the agent event stream while
// preserving field encapsulation.
func NewBaseEvent(level agent.AgentLevel, sessionID, runID, parentRunID string, ts time.Time) BaseEvent {
	return newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts)
}

// NewBaseEventFull constructs a BaseEvent with all fields including causal chain.
func NewBaseEventFull(level agent.AgentLevel, sessionID, runID, parentRunID, correlationID, causationID string, seq uint64, ts time.Time) BaseEvent {
	return BaseEvent{
		eventID:       id.NewEventID(),
		seq:           seq,
		timestamp:     ts,
		agentLevel:    level,
		sessionID:     sessionID,
		runID:         runID,
		parentRunID:   parentRunID,
		correlationID: correlationID,
		causationID:   causationID,
	}
}

// SetSeq assigns a sequence number to the event (typically called by emitter).
func (e *BaseEvent) SetSeq(seq uint64) { e.seq = seq }

// SetCorrelationID assigns the correlation ID.
func (e *BaseEvent) SetCorrelationID(cid string) { e.correlationID = cid }

// SetCausationID assigns the causation ID.
func (e *BaseEvent) SetCausationID(cid string) { e.causationID = cid }

// WorkflowInputReceivedEvent - emitted when a user submits a new task
type WorkflowInputReceivedEvent struct {
	BaseEvent
	Task        string
	Attachments map[string]ports.Attachment
}

func (e *WorkflowInputReceivedEvent) EventType() string { return "workflow.input.received" }

// GetAttachments exposes input attachments for attachment-aware listeners.
func (e *WorkflowInputReceivedEvent) GetAttachments() map[string]ports.Attachment {
	return ports.CloneAttachmentMap(e.Attachments)
}

// NewWorkflowInputReceivedEvent constructs a user task event with the provided metadata.
func NewWorkflowInputReceivedEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	task string,
	attachments map[string]ports.Attachment,
	ts time.Time,
) *WorkflowInputReceivedEvent {
	var cloned map[string]ports.Attachment
	if len(attachments) > 0 {
		cloned = ports.CloneAttachmentMap(attachments)
	}

	return &WorkflowInputReceivedEvent{
		BaseEvent:   newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
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
	Metadata      map[string]any
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
	Duration        time.Duration
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

// GetAttachments exposes tool result attachments for attachment-aware listeners.
func (e *WorkflowToolCompletedEvent) GetAttachments() map[string]ports.Attachment {
	return ports.CloneAttachmentMap(e.Attachments)
}

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
	SessionStats   *storage.SessionStats // Optional: session-level cost/token accumulation
	Attachments    map[string]ports.Attachment
}

func (e *WorkflowResultFinalEvent) EventType() string { return "workflow.result.final" }

// GetAttachments exposes final-result attachments for attachment-aware listeners.
func (e *WorkflowResultFinalEvent) GetAttachments() map[string]ports.Attachment {
	return ports.CloneAttachmentMap(e.Attachments)
}

// WorkflowResultCancelledEvent - emitted when a running task receives an explicit cancellation request
type WorkflowResultCancelledEvent struct {
	BaseEvent
	Reason      string
	RequestedBy string
}

func (e *WorkflowResultCancelledEvent) EventType() string { return "workflow.result.cancelled" }

// NewWorkflowResultCancelledEvent constructs a cancellation notification event for SSE consumers.
func NewWorkflowResultCancelledEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	reason, requestedBy string,
	ts time.Time,
) *WorkflowResultCancelledEvent {
	return &WorkflowResultCancelledEvent{
		BaseEvent:   newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
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

// WorkflowPreAnalysisEmojiEvent - emitted when pre-analysis determines a react emoji
type WorkflowPreAnalysisEmojiEvent struct {
	BaseEvent
	ReactEmoji string
}

func (e *WorkflowPreAnalysisEmojiEvent) EventType() string {
	return "workflow.diagnostic.preanalysis_emoji"
}

// NewWorkflowPreAnalysisEmojiEvent constructs a pre-analysis emoji event.
func NewWorkflowPreAnalysisEmojiEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	reactEmoji string,
	ts time.Time,
) *WorkflowPreAnalysisEmojiEvent {
	return &WorkflowPreAnalysisEmojiEvent{
		BaseEvent:  newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		ReactEmoji: reactEmoji,
	}
}

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
func NewWorkflowDiagnosticContextCompressionEvent(level agent.AgentLevel, sessionID, runID, parentRunID string, originalCount, compressedCount int, ts time.Time) *WorkflowDiagnosticContextCompressionEvent {
	compressionRate := 0.0
	if originalCount > 0 {
		compressionRate = float64(compressedCount) / float64(originalCount) * 100.0
	}
	return &WorkflowDiagnosticContextCompressionEvent{
		BaseEvent:       newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
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
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	iteration int,
	llmTurnSeq int,
	requestID string,
	messages, excluded []ports.Message,
	ts time.Time,
) *WorkflowDiagnosticContextSnapshotEvent {
	return &WorkflowDiagnosticContextSnapshotEvent{
		BaseEvent:  newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
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
func NewWorkflowDiagnosticToolFilteringEvent(level agent.AgentLevel, sessionID, runID, parentRunID, presetName string, originalCount, filteredCount int, filteredTools []string, ts time.Time) *WorkflowDiagnosticToolFilteringEvent {
	filterRatio := 0.0
	if originalCount > 0 {
		filterRatio = float64(filteredCount) / float64(originalCount) * 100.0
	}
	return &WorkflowDiagnosticToolFilteringEvent{
		BaseEvent:       newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
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
		BaseEvent: newBaseEventWithIDs(agent.LevelCore, "", "", "", captured),
		Host:      cloneStringMap(host),
		Captured:  captured,
	}
}

// ProactiveContextRefreshEvent signals a mid-loop proactive memory refresh.
type ProactiveContextRefreshEvent struct {
	BaseEvent
	Iteration        int
	MemoriesInjected int
}

func (e *ProactiveContextRefreshEvent) EventType() string {
	return "proactive.context.refresh"
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

// BackgroundTaskDispatchedEvent is emitted when a background task is dispatched.
type BackgroundTaskDispatchedEvent struct {
	BaseEvent
	TaskID      string
	Description string
	Prompt      string
	AgentType   string
}

func (e *BackgroundTaskDispatchedEvent) EventType() string {
	return "background.task.dispatched"
}

// BackgroundTaskCompletedEvent is emitted when a background task finishes.
type BackgroundTaskCompletedEvent struct {
	BaseEvent
	TaskID      string
	Description string
	Status      string // "completed", "failed", "cancelled"
	Answer      string
	Error       string
	Duration    time.Duration
	Iterations  int
	TokensUsed  int
}

func (e *BackgroundTaskCompletedEvent) EventType() string {
	return "background.task.completed"
}

// EventListenerFunc is a function adapter for EventListener
type EventListenerFunc func(AgentEvent)

func (f EventListenerFunc) OnEvent(event AgentEvent) {
	f(event)
}
