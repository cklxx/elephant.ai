package domain

import (
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/workflow"
	"time"
)

// Re-export the event listener contract defined at the port layer.
type AgentEvent = agent.AgentEvent
type EventListener = agent.EventListener

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
	Iteration   int                         `json:"iteration,omitempty"`
	Content     string                      `json:"content,omitempty"`
	Metadata    map[string]any              `json:"metadata,omitempty"`
	Attachments map[string]ports.Attachment `json:"attachments,omitempty"`

	// --- Input --------------------------------------------------------------
	Task string `json:"task,omitempty"` // EventInputReceived

	// --- Node lifecycle -----------------------------------------------------
	TotalIters      int                        `json:"total_iters,omitempty"`
	StepIndex       int                        `json:"step_index,omitempty"`
	StepDescription string                     `json:"step_description,omitempty"`
	Input           any                        `json:"input,omitempty"`
	Workflow        *workflow.WorkflowSnapshot `json:"workflow,omitempty"`
	StepResult      any                        `json:"step_result,omitempty"`
	Status          string                     `json:"status,omitempty"`
	TokensUsed      int                        `json:"tokens_used,omitempty"`
	ToolsRun        int                        `json:"tools_run,omitempty"`
	Duration        time.Duration              `json:"duration,omitempty"`

	// --- Node output delta --------------------------------------------------
	MessageCount int       `json:"message_count,omitempty"`
	Delta        string    `json:"delta,omitempty"`
	Final        bool      `json:"final,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	SourceModel  string    `json:"source_model,omitempty"`

	// --- Node output summary ------------------------------------------------
	ToolCallCount int `json:"tool_call_count,omitempty"`

	// --- Lifecycle updated --------------------------------------------------
	WorkflowID        string                 `json:"workflow_id,omitempty"`
	WorkflowEventType workflow.EventType     `json:"workflow_event_type,omitempty"`
	Phase             workflow.WorkflowPhase `json:"phase,omitempty"`
	Node              *workflow.NodeSnapshot `json:"node,omitempty"`

	// --- Tool lifecycle -----------------------------------------------------
	CallID    string                 `json:"call_id,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Result    string                 `json:"result,omitempty"`
	Error     error                  `json:"-"`               // not JSON-serialized
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
	LLMTurnSeq      int    `json:"llm_turn_seq,omitempty"`
	RequestID       string `json:"request_id,omitempty"`
	ContextMsgCount int    `json:"context_msg_count,omitempty"`
	ExcludedCount   int    `json:"excluded_count,omitempty"`
	ContextPreview  string `json:"context_preview,omitempty"` // summary of first/last messages

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
	MergeStatus string `json:"merge_status,omitempty"`
	Iterations  int    `json:"iterations,omitempty"`

	// --- External agent progress --------------------------------------------
	MaxIter      int           `json:"max_iter,omitempty"`
	CostUSD      float64       `json:"cost_usd,omitempty"`
	CurrentTool  string        `json:"current_tool,omitempty"`
	CurrentArgs  string        `json:"current_args,omitempty"`
	FilesTouched []string      `json:"files_touched,omitempty"`
	LastActivity time.Time     `json:"last_activity,omitempty"`
	Elapsed      time.Duration `json:"elapsed,omitempty"`

	// --- External input request / response ----------------------------------
	Type     string `json:"type,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Approved bool   `json:"approved,omitempty"`
	OptionID string `json:"option_id,omitempty"`
	Message  string `json:"message,omitempty"`
}

// NewEvent constructs an Event with the given kind and base.
func NewEvent(kind EventKind, base BaseEvent) *Event {
	return &Event{BaseEvent: base, Kind: kind}
}

// ---------------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------------

func percentageOf(value, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100.0
}

// EventListenerFunc is a function adapter for EventListener
type EventListenerFunc func(AgentEvent)

func (f EventListenerFunc) OnEvent(event AgentEvent) {
	f(event)
}
