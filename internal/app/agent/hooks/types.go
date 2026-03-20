package hooks

import (
	"context"
)

// injectionType classifies the kind of content being injected into context.
type injectionType string

const (
	injectionSuggestion injectionType = "suggestion"
	injectionWarning    injectionType = "warning"
	injectionOKRContext injectionType = "okr_context"
)

// Injection describes content to be injected into the agent context.
type Injection struct {
	Type     injectionType // Classification of the injection
	Content  string        // Text content to inject
	Source   string        // Name of the hook that produced this injection
	Priority int           // Higher priority injections appear first
}

// TaskInfo provides task context to hooks without coupling to domain types.
type TaskInfo struct {
	TaskInput   string
	SessionID   string
	RunID       string
	UserID      string
	ToolResults []ToolResultInfo
}

// ToolResultInfo is a simplified tool result for hook consumption.
type ToolResultInfo struct {
	ToolName string
	Success  bool
	Output   string
}

// TaskResultInfo provides task completion context to hooks.
type TaskResultInfo struct {
	TaskInput  string
	Answer     string
	SessionID  string
	RunID      string
	UserID     string
	Iterations int
	StopReason string
	ToolCalls  []ToolResultInfo
}

// ProactiveHook defines the interface for proactive behavior hooks.
// Implementations can inject content into agent context at various lifecycle points.
type ProactiveHook interface {
	// Name returns a unique identifier for this hook (used in observability).
	Name() string

	// OnTaskStart is called before task execution begins.
	// Returns injections to prepend to the agent context.
	OnTaskStart(ctx context.Context, task TaskInfo) []Injection

	// OnTaskCompleted is called after task execution finishes.
	// Used for post-processing such as audit or metrics.
	OnTaskCompleted(ctx context.Context, result TaskResultInfo) error
}

// FormatInjectionsAsContext formats injections into a text block suitable
// for injection into the agent system prompt or as a user message.
func FormatInjectionsAsContext(injections []Injection) string {
	if len(injections) == 0 {
		return ""
	}

	var buf []byte
	buf = append(buf, "## Proactive Context\n\n"...)
	for _, inj := range injections {
		buf = append(buf, "### "...)
		buf = append(buf, string(inj.Type)...)
		buf = append(buf, " (from "...)
		buf = append(buf, inj.Source...)
		buf = append(buf, ")\n\n"...)
		buf = append(buf, inj.Content...)
		buf = append(buf, "\n\n"...)
	}
	return string(buf)
}
