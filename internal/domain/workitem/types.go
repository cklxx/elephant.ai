// Package workitem defines canonical domain types for external work items
// (Jira issues, Linear issues) consumed by leader agent features such as
// Blocker Radar, Prep Brief, Weekly Pulse, and Scope Change Detection.
//
// These types are deliberately separate from internal/domain/task — task.Task
// represents work elephant.ai owns, while workitem.WorkItem represents work
// tracked in external project management systems.
package workitem

import "time"

// Provider identifies the external project management system.
type Provider string

const (
	ProviderJira   Provider = "jira"
	ProviderLinear Provider = "linear"
)

// StatusClass normalizes provider-specific statuses into a small canonical set.
type StatusClass string

const (
	StatusTodo       StatusClass = "todo"
	StatusInProgress StatusClass = "in_progress"
	StatusBlocked    StatusClass = "blocked"
	StatusDone       StatusClass = "done"
	StatusCancelled  StatusClass = "cancelled"
	StatusUnknown    StatusClass = "unknown"
)

// IsTerminal reports whether the status represents a final state.
func (s StatusClass) IsTerminal() bool {
	return s == StatusDone || s == StatusCancelled
}

// PersonRef identifies a person in an external system.
type PersonRef struct {
	ExternalID  string `json:"external_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
}

// WorkItem is the canonical representation of an external issue or ticket.
type WorkItem struct {
	// Identity
	ID          string   `json:"id"`
	Provider    Provider `json:"provider"`
	WorkspaceID string   `json:"workspace_id"`
	ProjectID   string   `json:"project_id"`
	ProjectKey  string   `json:"project_key"`
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	URL         string   `json:"url"`

	// Ownership
	Assignee PersonRef `json:"assignee"`
	Reporter PersonRef `json:"reporter"`

	// Lifecycle
	StatusID    string      `json:"status_id"`
	StatusName  string      `json:"status_name"`
	StatusClass StatusClass `json:"status_class"`

	// Risk signals
	Priority      string   `json:"priority"`
	Labels        []string `json:"labels,omitempty"`
	IsBlocked     bool     `json:"is_blocked"`
	BlockedReason string   `json:"blocked_reason,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Audit
	SourceVersion string `json:"source_version,omitempty"`

	// Provider-specific metadata for fields not yet normalized.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Comment is a normalized issue comment from an external system.
type Comment struct {
	ID          string   `json:"id"`
	Provider    Provider `json:"provider"`
	WorkspaceID string   `json:"workspace_id"`
	WorkItemID  string   `json:"work_item_id"`
	Author      PersonRef `json:"author"`
	BodyText    string   `json:"body_text"`
	BodyRaw     string   `json:"body_raw,omitempty"`
	IsSystem    bool     `json:"is_system"`
	Visibility  string   `json:"visibility,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// StatusChange records a transition between statuses for an external work item.
type StatusChange struct {
	ID             string    `json:"id"`
	Provider       Provider  `json:"provider"`
	WorkspaceID    string    `json:"workspace_id"`
	WorkItemID     string    `json:"work_item_id"`
	FromStatusID   string    `json:"from_status_id"`
	FromStatusName string    `json:"from_status_name"`
	ToStatusID     string    `json:"to_status_id"`
	ToStatusName   string    `json:"to_status_name"`
	ChangedBy      PersonRef `json:"changed_by"`
	ChangedAt      time.Time `json:"changed_at"`
	Source         string    `json:"source"` // "changelog", "webhook_diff", "polling_diff"
}

// IssueFilter specifies criteria for querying work items.
type IssueFilter struct {
	Provider     Provider      `json:"provider"`
	WorkspaceID  string        `json:"workspace_id,omitempty"`
	ProjectIDs   []string      `json:"project_ids,omitempty"`
	AssigneeIDs  []string      `json:"assignee_ids,omitempty"`
	Statuses     []StatusClass `json:"statuses,omitempty"`
	UpdatedAfter time.Time     `json:"updated_after,omitempty"`
	Limit        int           `json:"limit,omitempty"`
}
