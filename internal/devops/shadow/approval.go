package shadow

import (
	"context"
	"fmt"

	tools "alex/internal/agent/ports/tools"
)

// RequireApproval enforces manual approval for shadow agent execution.
func RequireApproval(ctx context.Context, approver tools.Approver, task Task) error {
	if approver == nil {
		return fmt.Errorf("shadow agent approval required")
	}

	req := &tools.ApprovalRequest{
		Operation:  "shadow_agent_execute",
		Summary:    fmt.Sprintf("Approval required for shadow agent task %s", task.ID),
		ToolName:   "shadow_agent",
		ToolCallID: task.ID,
		Arguments: map[string]any{
			"summary": task.Summary,
			"prompt":  task.Prompt,
		},
	}

	resp, err := approver.RequestApproval(ctx, req)
	if err != nil {
		return err
	}
	if resp == nil || !resp.Approved {
		return fmt.Errorf("shadow agent task rejected")
	}
	return nil
}
