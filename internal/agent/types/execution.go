package types

// AgentTask represents a task record exposed via public APIs and streaming channels.
type AgentTask struct {
	TaskID       string  `json:"task_id"`
	SessionID    string  `json:"session_id"`
	ParentTaskID string  `json:"parent_task_id,omitempty"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	CompletedAt  *string `json:"completed_at,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// ExecutionEvent captures an emitted agent event for external consumers.
type ExecutionEvent struct {
	EventType    string         `json:"event_type"`
	Timestamp    string         `json:"timestamp"`
	SessionID    string         `json:"session_id"`
	TaskID       string         `json:"task_id"`
	ParentTaskID string         `json:"parent_task_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
}
