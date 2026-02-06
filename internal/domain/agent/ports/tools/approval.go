package tools

import "context"

// ApprovalRequest contains information for requesting user approval
type ApprovalRequest struct {
	Operation       string // "file_edit", "file_write", "file_delete"
	FilePath        string
	Diff            string
	Summary         string
	AutoApprove     bool
	ToolCallID      string
	ToolName        string
	Arguments       map[string]any
	SafetyLevel     int    `json:"safety_level,omitempty"`     // L1-L4; 0=unset
	RollbackSteps   string `json:"rollback_steps,omitempty"`   // how to undo (L3+)
	AlternativePlan string `json:"alternative_plan,omitempty"` // safer alternative (L4)
}

// ApprovalResponse contains the user's approval decision
type ApprovalResponse struct {
	Approved bool
	Action   string // "approve", "reject", "edit", "quit"
	Message  string
}

// Approver handles approval requests for dangerous operations
type Approver interface {
	// RequestApproval asks for user approval for an operation
	RequestApproval(ctx context.Context, request *ApprovalRequest) (*ApprovalResponse, error)
}
