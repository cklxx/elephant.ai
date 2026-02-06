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

// ExecutionEvent captures an emitted agent event for external consumers.
type ExecutionEvent struct {
	EventType   string         `json:"event_type"`
	Timestamp   string         `json:"timestamp"`
	SessionID   string         `json:"session_id"`
	RunID       string         `json:"run_id"`
	ParentRunID string         `json:"parent_run_id,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}
