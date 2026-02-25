// Package kernel defines the domain types for the kernel agent loop.
// The kernel is a cron-driven orchestrator that periodically dispatches
// agent tasks, with state managed entirely in an opaque STATE.md file.
package kernel

import "time"

// DispatchStatus represents the lifecycle of a dispatch task.
type DispatchStatus string

const (
	DispatchPending   DispatchStatus = "pending"
	DispatchRunning   DispatchStatus = "running"
	DispatchDone      DispatchStatus = "done"
	DispatchFailed    DispatchStatus = "failed"
	DispatchCancelled DispatchStatus = "cancelled"
)

// CycleResultStatus summarises the outcome of a single RunCycle.
type CycleResultStatus string

const (
	CycleSuccess        CycleResultStatus = "success"
	CyclePartialSuccess CycleResultStatus = "partial_success"
	CycleFailed         CycleResultStatus = "failed"
)

// DispatchSpec is the planner's output: one unit of work to enqueue.
type DispatchSpec struct {
	AgentID  string            `json:"agent_id"`
	Prompt   string            `json:"prompt"`
	Priority int               `json:"priority"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Dispatch is the persisted row for a single dispatched task.
type Dispatch struct {
	DispatchID string            `json:"dispatch_id"`
	KernelID   string            `json:"kernel_id"`
	CycleID    string            `json:"cycle_id"`
	AgentID    string            `json:"agent_id"`
	Prompt     string            `json:"prompt"`
	Priority   int               `json:"priority"`
	Status     DispatchStatus    `json:"status"`
	LeaseOwner string            `json:"lease_owner,omitempty"`
	LeaseUntil *time.Time        `json:"lease_until,omitempty"`
	TaskID     string            `json:"task_id,omitempty"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// AgentCycleSummary captures one agent dispatch outcome in a cycle.
type AgentCycleSummary struct {
	AgentID string         `json:"agent_id"`
	TaskID  string         `json:"task_id,omitempty"`
	Status  DispatchStatus `json:"status"`
	Summary string         `json:"summary,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// CycleResult is the summary returned by Engine.RunCycle.
type CycleResult struct {
	CycleID      string              `json:"cycle_id"`
	KernelID     string              `json:"kernel_id"`
	Status       CycleResultStatus   `json:"status"`
	Dispatched   int                 `json:"dispatched"`
	Succeeded    int                 `json:"succeeded"`
	Failed       int                 `json:"failed"`
	FailedAgents []string            `json:"failed_agents,omitempty"`
	AgentSummary []AgentCycleSummary `json:"agent_summary,omitempty"`
	Duration     time.Duration       `json:"duration"`
}
