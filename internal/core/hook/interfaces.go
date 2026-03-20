package hook

import "context"

// SessionResolver resolves the session for an inbound request.
type SessionResolver interface {
	ResolveSession(ctx context.Context, state *TurnState) error
}

// StateLoader loads prior state (messages, memory, etc.) for the turn.
type StateLoader interface {
	LoadState(ctx context.Context, state *TurnState) error
}

// PromptBuilder constructs the prompt sent to the LLM.
type PromptBuilder interface {
	BuildPrompt(ctx context.Context, state *TurnState) (*Prompt, error)
}

// ModelRunner calls the LLM and returns its output.
type ModelRunner interface {
	RunModel(ctx context.Context, state *TurnState, prompt *Prompt) (*ModelOutput, error)
}

// StateSaver persists state after the model run.
type StateSaver interface {
	SaveState(ctx context.Context, state *TurnState) error
}

// OutboundRenderer renders model output into channel-specific outbound messages.
type OutboundRenderer interface {
	RenderOutbound(ctx context.Context, state *TurnState, output *ModelOutput) ([]Outbound, error)
}

// OutboundDispatcher sends rendered outbound messages to channels.
type OutboundDispatcher interface {
	DispatchOutbound(ctx context.Context, outbounds []Outbound) error
}

// PreTaskHook runs before the main model execution.
type PreTaskHook interface {
	PreTask(ctx context.Context, state *TurnState) error
}

// PostTaskHook runs after model execution completes.
type PostTaskHook interface {
	PostTask(ctx context.Context, state *TurnState, result *TurnResult) error
}

// ToolExecutionHook intercepts tool execution.
type ToolExecutionHook interface {
	BeforeToolExecution(ctx context.Context, toolName string, args map[string]any) (map[string]any, error)
	AfterToolExecution(ctx context.Context, toolName string, result string, err error) error
}

// ErrorHandler handles errors during the turn lifecycle.
type ErrorHandler interface {
	HandleError(ctx context.Context, state *TurnState, step string, err error) error
}

// IterationHook runs on each ReAct iteration.
type IterationHook interface {
	OnIteration(ctx context.Context, state *TurnState, iteration int) error
}

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

// TurnState is the mutable state bag for a single turn, replacing agent.TaskState.
type TurnState struct {
	SessionID     string
	RunID         string
	ParentRunID   string
	CorrelationID string
	UserID        string
	Channel       string
	Input         string
	Messages      []Message
	Metadata      map[string]any
	Values        map[string]any // arbitrary plugin state
}

// Set stores a value in the Values map, initializing the map if nil.
func (s *TurnState) Set(key string, value any) {
	if s.Values == nil {
		s.Values = make(map[string]any)
	}
	s.Values[key] = value
}

// Get retrieves a value from the Values map.
func (s *TurnState) Get(key string) (any, bool) {
	if s.Values == nil {
		return nil, false
	}
	v, ok := s.Values[key]
	return v, ok
}

// GetString retrieves a string from the Values map.
// Returns "" if the key is missing or the value is not a string.
func (s *TurnState) GetString(key string) string {
	v, ok := s.Get(key)
	if !ok {
		return ""
	}
	str, _ := v.(string)
	return str
}

// Message is a conversation message.
type Message struct {
	Role    string         `json:"role"`
	Content string         `json:"content"`
	Source  string         `json:"source,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// Prompt is a structured prompt for the LLM.
type Prompt struct {
	System   string
	Messages []Message
	Tools    []ToolSchema
}

// ToolSchema describes a tool for the LLM.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ModelOutput is the LLM response.
type ModelOutput struct {
	Text       string
	ToolCalls  []ToolCallOutput
	Usage      Usage
	StopReason string
	Model      string
}

// ToolCallOutput represents a tool call from the model.
type ToolCallOutput struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// Outbound is a rendered output message for a channel.
type Outbound struct {
	Channel   string
	SessionID string
	Content   string
	Media     []MediaItem
	Metadata  map[string]any
}

// MediaItem is a media attachment in an outbound message.
type MediaItem struct {
	Type string // "image", "file", "audio"
	URL  string
	Data []byte
	Name string
}

// TurnResult is the final result of a turn.
type TurnResult struct {
	SessionID   string
	RunID       string
	Input       string
	Prompt      *Prompt
	ModelOutput *ModelOutput
	Outbounds   []Outbound
	Error       error
	Metadata    map[string]any
}
