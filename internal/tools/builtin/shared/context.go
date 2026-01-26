package builtin

import (
	"context"

	tools "alex/internal/agent/ports/tools"
	"alex/internal/backup"
)

// Context keys for tool dependencies
type toolContextKey string

const (
	ApproverKey      toolContextKey = "approver"
	BackupManagerKey toolContextKey = "backup_manager"
	ToolSessionIDKey toolContextKey = "tool_session_id"
	AutoApproveKey   toolContextKey = "auto_approve"
)

// GetApproverFromContext retrieves the approver from context
func GetApproverFromContext(ctx context.Context) tools.Approver {
	if ctx == nil {
		return nil
	}

	if approver, ok := ctx.Value(ApproverKey).(tools.Approver); ok {
		return approver
	}

	return nil
}

// WithApprover sets the approver in context
func WithApprover(ctx context.Context, approver tools.Approver) context.Context {
	return context.WithValue(ctx, ApproverKey, approver)
}

// GetBackupManagerFromContext retrieves the backup manager from context
func GetBackupManagerFromContext(ctx context.Context) *backup.Manager {
	if ctx == nil {
		return nil
	}

	if manager, ok := ctx.Value(BackupManagerKey).(*backup.Manager); ok {
		return manager
	}

	return nil
}

// WithBackupManager sets the backup manager in context
func WithBackupManager(ctx context.Context, manager *backup.Manager) context.Context {
	return context.WithValue(ctx, BackupManagerKey, manager)
}

// GetToolSessionIDFromContext retrieves the session ID from context
func GetToolSessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if sessionID, ok := ctx.Value(ToolSessionIDKey).(string); ok {
		return sessionID
	}

	return ""
}

// WithToolSessionID sets the session ID in context
func WithToolSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, ToolSessionIDKey, sessionID)
}

// GetAutoApproveFromContext retrieves the auto-approve flag from context
func GetAutoApproveFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	if autoApprove, ok := ctx.Value(AutoApproveKey).(bool); ok {
		return autoApprove
	}

	return false
}

// WithAutoApprove sets the auto-approve flag in context
func WithAutoApprove(ctx context.Context, autoApprove bool) context.Context {
	return context.WithValue(ctx, AutoApproveKey, autoApprove)
}
