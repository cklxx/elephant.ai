package kernel

import "time"

// DispatchStatus represents the lifecycle state of a dispatch.
type DispatchStatus string

const (
	DispatchPending  DispatchStatus = "pending"
	DispatchRunning  DispatchStatus = "running"
	DispatchDone     DispatchStatus = "done"
	DispatchFailed   DispatchStatus = "failed"
	DispatchCancelled DispatchStatus = "cancelled"
)

// IsTerminal returns true when the dispatch has reached a final state.
func (s DispatchStatus) IsTerminal() bool {
	switch s {
	case DispatchDone, DispatchFailed, DispatchCancelled:
		return true
	default:
		return false
	}
}

// Dispatch represents a single unit of work dispatched by the kernel engine.
type Dispatch struct {
	DispatchID string         `json:"dispatch_id"`
	KernelID   string         `json:"kernel_id"`
	CycleID    string         `json:"cycle_id"`
	AgentName  string         `json:"agent_name"`
	Prompt     string         `json:"prompt"`
	Status     DispatchStatus `json:"status"`
	Error      string         `json:"error,omitempty"`
	Summary    string         `json:"summary,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// DispatchSpec is a planned dispatch produced by the planner.
type DispatchSpec struct {
	AgentName string `json:"agent_name"`
	Prompt    string `json:"prompt"`
}

// CycleHistoryEntry records the outcome of a single kernel cycle.
type CycleHistoryEntry struct {
	CycleID      string    `json:"cycle_id"`
	Timestamp    time.Time `json:"timestamp"`
	Dispatched   int       `json:"dispatched"`
	Succeeded    int       `json:"succeeded"`
	Failed       int       `json:"failed"`
	ErrorSummary string    `json:"error_summary,omitempty"`
}
