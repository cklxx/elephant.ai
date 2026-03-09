package types

// AgentTask represents a task record exposed via public APIs and streaming channels.
type AgentTask struct {
	RunID       string  `json:"run_id"`
	SessionID   string  `json:"session_id"`
	ParentRunID string  `json:"parent_run_id,omitempty"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
}
