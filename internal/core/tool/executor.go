package tool

import "context"

// Executor executes a tool call and returns its result.
type Executor interface {
	Execute(ctx context.Context, name string, args map[string]any) (string, error)
	Available() []Tool
}

// Result is the output of a tool execution.
type Result struct {
	CallID   string         `json:"call_id"`
	Content  string         `json:"content"`
	Error    error          `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
