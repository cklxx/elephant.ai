package shadow

import "alex/internal/infra/coding"

// Task defines a shadow agent coding task.
type Task struct {
	ID          string
	Summary     string
	Prompt      string
	AgentType   string
	WorkingDir  string
	Config      map[string]string
	SessionID   string
	CausationID string
}

// Result captures the shadow agent execution result.
type Result struct {
	TaskID   string
	Answer   string
	Error    string
	Metadata map[string]any
	Verify   *coding.VerifyResult
}
