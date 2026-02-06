package coding

import "time"

// TaskRequest describes a coding agent task request.
type TaskRequest struct {
	TaskID      string
	Prompt      string
	AgentType   string
	WorkingDir  string
	Config      map[string]string
	SessionID   string
	CausationID string
}

// TaskResult captures the outcome of a coding agent run.
type TaskResult struct {
	TaskID     string
	Answer     string
	Iterations int
	TokensUsed int
	Error      string
	Metadata   map[string]any
}

// TaskState tracks the current lifecycle state.
type TaskState string

const (
	TaskStateQueued    TaskState = "queued"
	TaskStateRunning   TaskState = "running"
	TaskStateSucceeded TaskState = "succeeded"
	TaskStateFailed    TaskState = "failed"
	TaskStateCanceled  TaskState = "canceled"
)

// TaskStatus provides a point-in-time status snapshot.
type TaskStatus struct {
	TaskID    string
	State     TaskState
	UpdatedAt time.Time
	Detail    string
}

// TaskProgress describes streaming progress updates.
type TaskProgress struct {
	Iteration    int
	TokensUsed   int
	CostUSD      float64
	CurrentTool  string
	CurrentArgs  string
	FilesTouched []string
	LastActivity time.Time
}

// ProgressCallback receives progress updates.
type ProgressCallback func(TaskProgress)
