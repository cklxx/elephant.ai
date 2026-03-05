package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

// ApprovalExecutor is a ToolExecutor decorator that gates dangerous tool
// execution behind an approver. For non-dangerous tools it delegates directly.
type ApprovalExecutor struct {
	delegate tools.ToolExecutor
}

// NewApprovalExecutor creates an ApprovalExecutor wrapping the given delegate.
func NewApprovalExecutor(delegate tools.ToolExecutor) tools.ToolExecutor {
	return &ApprovalExecutor{delegate: delegate}
}

// Delegate returns the inner executor for unwrapping.
func (a *ApprovalExecutor) Delegate() tools.ToolExecutor {
	return a.delegate
}

func (a *ApprovalExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if a.delegate == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("tool executor missing")}, nil
	}
	meta := a.delegate.Metadata()
	if !meta.Dangerous {
		return a.delegate.Execute(ctx, call)
	}
	approver := shared.GetApproverFromContext(ctx)
	if approver == nil || shared.GetAutoApproveFromContext(ctx) {
		return a.delegate.Execute(ctx, call)
	}

	req := &tools.ApprovalRequest{
		Operation:   meta.Name,
		FilePath:    extractFilePath(call.Arguments),
		Summary:     buildApprovalSummary(meta, call),
		AutoApprove: shared.GetAutoApproveFromContext(ctx),
		ToolCallID:  call.ID,
		ToolName:    call.Name,
		Arguments:   call.Arguments,
		SafetyLevel: meta.EffectiveSafetyLevel(),
	}
	if req.SafetyLevel >= ports.SafetyLevelHighImpact {
		req.RollbackSteps = buildRollbackSteps(meta, req.FilePath)
	}
	if req.SafetyLevel >= ports.SafetyLevelIrreversible {
		req.AlternativePlan = buildAlternativePlan(meta)
	}
	resp, err := approver.RequestApproval(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	if resp == nil || !resp.Approved {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("operation rejected")}, nil
	}

	return a.delegate.Execute(ctx, call)
}

func (a *ApprovalExecutor) Definition() ports.ToolDefinition {
	return a.delegate.Definition()
}

func (a *ApprovalExecutor) Metadata() ports.ToolMetadata {
	return a.delegate.Metadata()
}

func extractFilePath(args map[string]any) string {
	if args == nil {
		return ""
	}
	for _, key := range []string{"file_path", "path", "resolved_path"} {
		if val, ok := args[key].(string); ok {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func buildApprovalSummary(meta ports.ToolMetadata, call ports.ToolCall) string {
	parts := []string{fmt.Sprintf("Approval required for %s (L%d)", meta.Name, meta.EffectiveSafetyLevel())}
	if filePath := extractFilePath(call.Arguments); filePath != "" {
		parts = append(parts, fmt.Sprintf("path=%s", filePath))
	}
	if keys := extractArgumentKeys(call.Arguments); len(keys) > 0 {
		parts = append(parts, fmt.Sprintf("args=%s", strings.Join(keys, ", ")))
	}
	return strings.Join(parts, "; ")
}

func extractArgumentKeys(args map[string]any) []string {
	if len(args) == 0 {
		return nil
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		keys = append(keys[:8], "...")
	}
	return keys
}

func buildRollbackSteps(meta ports.ToolMetadata, filePath string) string {
	if filePath != "" {
		return fmt.Sprintf("If outcome is incorrect, restore %s from VCS/backups and rerun the last known-good step.", filePath)
	}
	return fmt.Sprintf("If outcome is incorrect, revert the %s operation via VCS/rollback tooling and rerun the last known-good step.", meta.Name)
}

func buildAlternativePlan(meta ports.ToolMetadata) string {
	if strings.Contains(strings.ToLower(meta.Name), "delete") {
		return "Prefer archive/disable first; verify impact in read-only mode before irreversible deletion."
	}
	return "Run a read-only or dry-run check first, then apply the smallest reversible change."
}

var _ tools.ToolExecutor = (*ApprovalExecutor)(nil)
