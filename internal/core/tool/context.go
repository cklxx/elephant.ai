package tool

// ToolContext provides immutable context for tool execution.
type ToolContext struct {
	TapeName  string
	RunID     string
	SessionID string
	Meta      map[string]string
	State     map[string]any
}

// NewToolContext creates a new ToolContext with initialized maps.
func NewToolContext(tapeName, runID, sessionID string) *ToolContext {
	return &ToolContext{
		TapeName:  tapeName,
		RunID:     runID,
		SessionID: sessionID,
		Meta:      make(map[string]string),
		State:     make(map[string]any),
	}
}
