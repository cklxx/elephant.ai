package domain

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/domain/workflow"
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

type eventIDProvider interface {
	NewEventID() string
}

type defaultEventIDProvider struct{}

func (defaultEventIDProvider) NewEventID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}

var (
	eventIDProviderMu sync.RWMutex
	currentEventIDGen eventIDProvider = defaultEventIDProvider{}
)

// SetEventIDGenerator installs the event ID generator used by domain events.
func SetEventIDGenerator(generator agent.IDGenerator) {
	if generator == nil {
		return
	}
	eventIDProviderMu.Lock()
	currentEventIDGen = generator
	eventIDProviderMu.Unlock()
}

// Next returns the next sequence number.
func (s *SeqCounter) Next() uint64 {
	return s.counter.Add(1)
}

func newBaseEventWithIDs(level agent.AgentLevel, sessionID, runID, parentRunID string, ts time.Time) BaseEvent {
	return BaseEvent{
		eventID:     nextEventID(),
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
		eventID:       nextEventID(),
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

func nextEventID() string {
	eventIDProviderMu.RLock()
	generator := currentEventIDGen
	eventIDProviderMu.RUnlock()
	if generator == nil {
		return defaultEventIDProvider{}.NewEventID()
	}
	return generator.NewEventID()
}

// ---------------------------------------------------------------------------
// Unified Event — replaces 25+ concrete event structs
// ---------------------------------------------------------------------------

// EventKind identifies the type of domain event.
// Values correspond to the string constants in types/events.go.
type EventKind = string

// Event is the single domain event type.
// It embeds BaseEvent for identity/metadata and carries a Kind discriminator
// plus a flat EventData payload that covers every event variant.
type Event struct {
	BaseEvent
	Kind EventKind
	Data EventData
}

// EventType satisfies the AgentEvent interface.
func (e *Event) EventType() string { return e.Kind }

// GetAttachments returns a cloned copy of the event's attachments (if any).
func (e *Event) GetAttachments() map[string]ports.Attachment {
	return ports.CloneAttachmentMap(e.Data.Attachments)
}

// EventData holds every payload field used across all event kinds.
// Only the fields relevant to a given Kind are populated; the rest remain at
// their zero values. This is a deliberate trade-off: a single flat struct is
// simpler and more type-safe than interface{}/map[string]any for core fields.
type EventData struct {
	// --- Shared / multi-kind ------------------------------------------------
	Iteration   int            `json:"iteration,omitempty"`
	Content     string         `json:"content,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Attachments map[string]ports.Attachment `json:"attachments,omitempty"`

	// --- Input --------------------------------------------------------------
	Task string `json:"task,omitempty"` // EventInputReceived

	// --- Node lifecycle -----------------------------------------------------
	TotalIters      int                       `json:"total_iters,omitempty"`
	StepIndex       int                       `json:"step_index,omitempty"`
	StepDescription string                    `json:"step_description,omitempty"`
	Input           any                       `json:"input,omitempty"`
	Workflow        *workflow.WorkflowSnapshot `json:"workflow,omitempty"`
	StepResult      any                       `json:"step_result,omitempty"`
	Status          string                    `json:"status,omitempty"`
	TokensUsed      int                       `json:"tokens_used,omitempty"`
	ToolsRun        int                       `json:"tools_run,omitempty"`
	Duration        time.Duration             `json:"duration,omitempty"`

	// --- Node output delta --------------------------------------------------
	MessageCount int       `json:"message_count,omitempty"`
	Delta        string    `json:"delta,omitempty"`
	Final        bool      `json:"final,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	SourceModel  string    `json:"source_model,omitempty"`

	// --- Node output summary ------------------------------------------------
	ToolCallCount int `json:"tool_call_count,omitempty"`

	// --- Lifecycle updated --------------------------------------------------
	WorkflowID        string                  `json:"workflow_id,omitempty"`
	WorkflowEventType workflow.EventType      `json:"workflow_event_type,omitempty"`
	Phase             workflow.WorkflowPhase  `json:"phase,omitempty"`
	Node              *workflow.NodeSnapshot  `json:"node,omitempty"`

	// --- Tool lifecycle -----------------------------------------------------
	CallID    string                 `json:"call_id,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Result    string                 `json:"result,omitempty"`
	Error     error                  `json:"-"` // not JSON-serialized
	ErrorStr  string                 `json:"error,omitempty"` // string representation for serialization

	// --- Tool progress ------------------------------------------------------
	Chunk      string `json:"chunk,omitempty"`
	IsComplete bool   `json:"is_complete,omitempty"`

	// --- Replan requested ---------------------------------------------------
	Reason string `json:"reason,omitempty"`

	// --- Result final -------------------------------------------------------
	FinalAnswer     string `json:"final_answer,omitempty"`
	TotalIterations int    `json:"total_iterations,omitempty"`
	TotalTokens     int    `json:"total_tokens,omitempty"`
	StopReason      string `json:"stop_reason,omitempty"`
	IsStreaming     bool   `json:"is_streaming,omitempty"`
	StreamFinished  bool   `json:"stream_finished,omitempty"`

	// --- Result cancelled ---------------------------------------------------
	RequestedBy string `json:"requested_by,omitempty"`

	// --- Node failed --------------------------------------------------------
	PhaseLabel  string `json:"phase_label,omitempty"`
	Recoverable bool   `json:"recoverable,omitempty"`

	// --- Pre-analysis emoji -------------------------------------------------
	ReactEmoji string `json:"react_emoji,omitempty"`

	// --- Diagnostic: context compression ------------------------------------
	OriginalCount   int     `json:"original_count,omitempty"`
	CompressedCount int     `json:"compressed_count,omitempty"`
	CompressionRate float64 `json:"compression_rate,omitempty"`

	// --- Diagnostic: context snapshot ---------------------------------------
	LLMTurnSeq int             `json:"llm_turn_seq,omitempty"`
	RequestID  string          `json:"request_id,omitempty"`
	Messages   []ports.Message `json:"messages,omitempty"`
	Excluded   []ports.Message `json:"excluded,omitempty"`

	// --- Diagnostic: tool filtering -----------------------------------------
	PresetName      string   `json:"preset_name,omitempty"`
	FilteredCount   int      `json:"filtered_count,omitempty"`
	FilteredTools   []string `json:"filtered_tools,omitempty"`
	ToolFilterRatio float64  `json:"tool_filter_ratio,omitempty"`

	// --- Diagnostic: environment snapshot ------------------------------------
	Host     map[string]string `json:"host,omitempty"`
	Captured time.Time         `json:"captured,omitempty"`

	// --- Diagnostic: context checkpoint -------------------------------------
	PrunedMessages  int `json:"pruned_messages,omitempty"`
	PrunedTokens    int `json:"pruned_tokens,omitempty"`
	SummaryTokens   int `json:"summary_tokens,omitempty"`
	RemainingTokens int `json:"remaining_tokens,omitempty"`

	// --- Proactive context refresh ------------------------------------------
	MemoriesInjected int `json:"memories_injected,omitempty"`

	// --- Background tasks ---------------------------------------------------
	TaskID      string `json:"task_id,omitempty"`
	Description string `json:"description,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	AgentType   string `json:"agent_type,omitempty"`
	Answer      string `json:"answer,omitempty"`
	Iterations  int    `json:"iterations,omitempty"`

	// --- External agent progress --------------------------------------------
	MaxIter      int       `json:"max_iter,omitempty"`
	CostUSD      float64   `json:"cost_usd,omitempty"`
	CurrentTool  string    `json:"current_tool,omitempty"`
	CurrentArgs  string    `json:"current_args,omitempty"`
	FilesTouched []string  `json:"files_touched,omitempty"`
	LastActivity time.Time `json:"last_activity,omitempty"`
	Elapsed      time.Duration `json:"elapsed,omitempty"`

	// --- External input request / response ----------------------------------
	Type     string `json:"type,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Approved bool   `json:"approved,omitempty"`
	OptionID string `json:"option_id,omitempty"`
	Message  string `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// Constructors — named helpers for ergonomic event creation at emit sites
// ---------------------------------------------------------------------------

// NewEvent constructs an Event with the given kind and base.
func NewEvent(kind EventKind, base BaseEvent) *Event {
	return &Event{BaseEvent: base, Kind: kind}
}

// NewInputReceivedEvent constructs a user task event.
func NewInputReceivedEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	task string,
	attachments map[string]ports.Attachment,
	ts time.Time,
) *Event {
	var cloned map[string]ports.Attachment
	if len(attachments) > 0 {
		cloned = ports.CloneAttachmentMap(attachments)
	}
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventInputReceived,
		Data: EventData{
			Task:        task,
			Attachments: cloned,
		},
	}
}

// NewNodeStartedEvent constructs a node started event.
func NewNodeStartedEvent(base BaseEvent, iteration, totalIters, stepIndex int, stepDescription string, input any, wf *workflow.WorkflowSnapshot) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeStarted,
		Data: EventData{
			Iteration:       iteration,
			TotalIters:      totalIters,
			StepIndex:       stepIndex,
			StepDescription: stepDescription,
			Input:           input,
			Workflow:        wf,
		},
	}
}

// NewNodeOutputDeltaEvent constructs a streaming output delta event.
func NewNodeOutputDeltaEvent(base BaseEvent, iteration, messageCount int, delta string, final bool, createdAt time.Time, sourceModel string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeOutputDelta,
		Data: EventData{
			Iteration:    iteration,
			MessageCount: messageCount,
			Delta:        delta,
			Final:        final,
			CreatedAt:    createdAt,
			SourceModel:  sourceModel,
		},
	}
}

// NewNodeOutputSummaryEvent constructs a node output summary event.
func NewNodeOutputSummaryEvent(base BaseEvent, iteration int, content string, toolCallCount int, metadata map[string]any) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeOutputSummary,
		Data: EventData{
			Iteration:     iteration,
			Content:       content,
			ToolCallCount: toolCallCount,
			Metadata:      metadata,
		},
	}
}

// NewLifecycleUpdatedEvent constructs a workflow lifecycle updated event.
func NewLifecycleUpdatedEvent(base BaseEvent, workflowID string, wfEventType workflow.EventType, phase workflow.WorkflowPhase, node *workflow.NodeSnapshot, wf *workflow.WorkflowSnapshot) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventLifecycleUpdated,
		Data: EventData{
			WorkflowID:        workflowID,
			WorkflowEventType: wfEventType,
			Phase:             phase,
			Node:              node,
			Workflow:          wf,
		},
	}
}

// NewNodeCompletedEvent constructs a node completed event.
func NewNodeCompletedEvent(base BaseEvent, stepIndex int, stepDescription string, stepResult any, status string, iteration, tokensUsed, toolsRun int, duration time.Duration, wf *workflow.WorkflowSnapshot) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeCompleted,
		Data: EventData{
			StepIndex:       stepIndex,
			StepDescription: stepDescription,
			StepResult:      stepResult,
			Status:          status,
			Iteration:       iteration,
			TokensUsed:      tokensUsed,
			ToolsRun:        toolsRun,
			Duration:        duration,
			Workflow:        wf,
		},
	}
}

// NewToolStartedEvent constructs a tool started event.
func NewToolStartedEvent(base BaseEvent, iteration int, callID, toolName string, arguments map[string]interface{}) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventToolStarted,
		Data: EventData{
			Iteration: iteration,
			CallID:    callID,
			ToolName:  toolName,
			Arguments: arguments,
		},
	}
}

// NewToolProgressEvent constructs a tool progress event.
func NewToolProgressEvent(base BaseEvent, callID, chunk string, isComplete bool) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventToolProgress,
		Data: EventData{
			CallID:     callID,
			Chunk:      chunk,
			IsComplete: isComplete,
		},
	}
}

// NewToolCompletedEvent constructs a tool completed event.
func NewToolCompletedEvent(base BaseEvent, callID, toolName, result string, err error, duration time.Duration, metadata map[string]any, attachments map[string]ports.Attachment) *Event {
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	return &Event{
		BaseEvent: base,
		Kind:      types.EventToolCompleted,
		Data: EventData{
			CallID:      callID,
			ToolName:    toolName,
			Result:      result,
			Error:       err,
			ErrorStr:    errStr,
			Duration:    duration,
			Metadata:    metadata,
			Attachments: attachments,
		},
	}
}

// NewReplanRequestedEvent constructs a replan requested event.
func NewReplanRequestedEvent(base BaseEvent, callID, toolName, reason, errMsg string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventReplanRequested,
		Data: EventData{
			CallID:   callID,
			ToolName: toolName,
			Reason:   reason,
			ErrorStr: errMsg,
		},
	}
}

// NewResultFinalEvent constructs a result final event.
func NewResultFinalEvent(base BaseEvent, finalAnswer string, totalIterations, totalTokens int, stopReason string, duration time.Duration, isStreaming, streamFinished bool, attachments map[string]ports.Attachment) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventResultFinal,
		Data: EventData{
			FinalAnswer:     finalAnswer,
			TotalIterations: totalIterations,
			TotalTokens:     totalTokens,
			StopReason:      stopReason,
			Duration:        duration,
			IsStreaming:     isStreaming,
			StreamFinished:  streamFinished,
			Attachments:     attachments,
		},
	}
}

// NewResultCancelledEvent constructs a cancellation notification event.
func NewResultCancelledEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	reason, requestedBy string,
	ts time.Time,
) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventResultCancelled,
		Data: EventData{
			Reason:      reason,
			RequestedBy: requestedBy,
		},
	}
}

// NewNodeFailedEvent constructs a node failed event.
func NewNodeFailedEvent(base BaseEvent, iteration int, phase string, err error, recoverable bool) *Event {
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeFailed,
		Data: EventData{
			Iteration:   iteration,
			PhaseLabel:  phase,
			Error:       err,
			ErrorStr:    errStr,
			Recoverable: recoverable,
		},
	}
}

// NewPreAnalysisEmojiEvent constructs a pre-analysis emoji event.
func NewPreAnalysisEmojiEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	reactEmoji string,
	ts time.Time,
) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticPreanalysisEmoji,
		Data: EventData{
			ReactEmoji: reactEmoji,
		},
	}
}

// NewDiagnosticContextCompressionEvent creates a new context compression event.
func NewDiagnosticContextCompressionEvent(level agent.AgentLevel, sessionID, runID, parentRunID string, originalCount, compressedCount int, ts time.Time) *Event {
	compressionRate := 0.0
	if originalCount > 0 {
		compressionRate = float64(compressedCount) / float64(originalCount) * 100.0
	}
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticContextCompression,
		Data: EventData{
			OriginalCount:   originalCount,
			CompressedCount: compressedCount,
			CompressionRate: compressionRate,
		},
	}
}

// NewDiagnosticContextSnapshotEvent creates an immutable snapshot of the LLM context payload.
func NewDiagnosticContextSnapshotEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	iteration int,
	llmTurnSeq int,
	requestID string,
	messages, excluded []ports.Message,
	ts time.Time,
) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticContextSnapshot,
		Data: EventData{
			Iteration:  iteration,
			LLMTurnSeq: llmTurnSeq,
			RequestID:  requestID,
			Messages:   cloneMessageSlice(messages),
			Excluded:   cloneMessageSlice(excluded),
		},
	}
}

// NewDiagnosticToolFilteringEvent creates a new tool filtering event.
func NewDiagnosticToolFilteringEvent(level agent.AgentLevel, sessionID, runID, parentRunID, presetName string, originalCount, filteredCount int, filteredTools []string, ts time.Time) *Event {
	filterRatio := 0.0
	if originalCount > 0 {
		filterRatio = float64(filteredCount) / float64(originalCount) * 100.0
	}
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticToolFiltering,
		Data: EventData{
			PresetName:      presetName,
			OriginalCount:   originalCount,
			FilteredCount:   filteredCount,
			FilteredTools:   filteredTools,
			ToolFilterRatio: filterRatio,
		},
	}
}

// NewDiagnosticEnvironmentSnapshotEvent constructs a new environment snapshot event.
func NewDiagnosticEnvironmentSnapshotEvent(host map[string]string, captured time.Time) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(agent.LevelCore, "", "", "", captured),
		Kind:      types.EventDiagnosticEnvironmentSnapshot,
		Data: EventData{
			Host:     cloneStringMap(host),
			Captured: captured,
		},
	}
}

// NewDiagnosticContextCheckpointEvent constructs a context checkpoint event.
func NewDiagnosticContextCheckpointEvent(base BaseEvent, phaseLabel string, prunedMessages, prunedTokens, summaryTokens, remainingTokens int) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventDiagnosticContextCheckpoint,
		Data: EventData{
			PhaseLabel:      phaseLabel,
			PrunedMessages:  prunedMessages,
			PrunedTokens:    prunedTokens,
			SummaryTokens:   summaryTokens,
			RemainingTokens: remainingTokens,
		},
	}
}

// NewProactiveContextRefreshEvent constructs a proactive context refresh event.
func NewProactiveContextRefreshEvent(base BaseEvent, iteration, memoriesInjected int) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventProactiveContextRefresh,
		Data: EventData{
			Iteration:        iteration,
			MemoriesInjected: memoriesInjected,
		},
	}
}

// NewBackgroundTaskDispatchedEvent constructs a background task dispatched event.
func NewBackgroundTaskDispatchedEvent(base BaseEvent, taskID, description, prompt, agentType string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventBackgroundTaskDispatched,
		Data: EventData{
			TaskID:      taskID,
			Description: description,
			Prompt:      prompt,
			AgentType:   agentType,
		},
	}
}

// NewBackgroundTaskCompletedEvent constructs a background task completed event.
func NewBackgroundTaskCompletedEvent(base BaseEvent, taskID, description, status, answer, errMsg string, duration time.Duration, iterations, tokensUsed int) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventBackgroundTaskCompleted,
		Data: EventData{
			TaskID:      taskID,
			Description: description,
			Status:      status,
			Answer:      answer,
			ErrorStr:    errMsg,
			Duration:    duration,
			Iterations:  iterations,
			TokensUsed:  tokensUsed,
		},
	}
}

// NewExternalAgentProgressEvent constructs an external agent progress event.
func NewExternalAgentProgressEvent(base BaseEvent, taskID, agentType string, iteration, maxIter, tokensUsed int, costUSD float64, currentTool, currentArgs string, filesTouched []string, lastActivity time.Time, elapsed time.Duration) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventExternalAgentProgress,
		Data: EventData{
			TaskID:       taskID,
			AgentType:    agentType,
			Iteration:    iteration,
			MaxIter:      maxIter,
			TokensUsed:   tokensUsed,
			CostUSD:      costUSD,
			CurrentTool:  currentTool,
			CurrentArgs:  currentArgs,
			FilesTouched: filesTouched,
			LastActivity: lastActivity,
			Elapsed:      elapsed,
		},
	}
}

// NewExternalInputRequestEvent constructs an external input request event.
func NewExternalInputRequestEvent(base BaseEvent, taskID, agentType, requestID, reqType, summary string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventExternalInputRequested,
		Data: EventData{
			TaskID:    taskID,
			AgentType: agentType,
			RequestID: requestID,
			Type:      reqType,
			Summary:   summary,
		},
	}
}

// NewExternalInputResponseEvent constructs an external input response event.
func NewExternalInputResponseEvent(base BaseEvent, taskID, requestID string, approved bool, optionID, message string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventExternalInputResponded,
		Data: EventData{
			TaskID:    taskID,
			RequestID: requestID,
			Approved:  approved,
			OptionID:  optionID,
			Message:   message,
		},
	}
}

// ---------------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------------

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
