package shared

import (
	"context"

	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/backup"
)

// Context keys for tool dependencies
type toolContextKey string

const (
	ApproverKey      toolContextKey = "approver"
	BackupManagerKey toolContextKey = "backup_manager"
	ToolSessionIDKey toolContextKey = "tool_session_id"
	AutoApproveKey   toolContextKey = "auto_approve"
	larkClientKey    toolContextKey = "lark_client"
	larkChatIDKey    toolContextKey = "lark_chat_id"
	larkMessageIDKey toolContextKey = "lark_message_id"
	larkOAuthKey     toolContextKey = "lark_oauth"
	larkTenantCalKey toolContextKey = "lark_tenant_calendar_id"
	timerManagerKey  toolContextKey = "timer_manager"
	schedulerKey     toolContextKey = "scheduler"
	autoUploadKey    toolContextKey = "auto_upload_config"
)

type parentListenerKey struct{}

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

// GetParentListenerFromContext retrieves the parent listener (if any) for subtask event forwarding.
func GetParentListenerFromContext(ctx context.Context) agent.EventListener {
	if ctx == nil {
		return nil
	}

	if listener := ctx.Value(parentListenerKey{}); listener != nil {
		if typed, ok := listener.(agent.EventListener); ok {
			return typed
		}
	}

	return nil
}

// WithParentListener adds a parent listener to context for subagent event forwarding.
func WithParentListener(ctx context.Context, listener agent.EventListener) context.Context {
	return context.WithValue(ctx, parentListenerKey{}, listener)
}

// WithLarkClient sets the Lark client in context. Typed as interface{} to avoid
// importing the Lark SDK in the shared package.
func WithLarkClient(ctx context.Context, client interface{}) context.Context {
	return context.WithValue(ctx, larkClientKey, client)
}

// LarkClientFromContext retrieves the Lark client from context.
func LarkClientFromContext(ctx context.Context) interface{} {
	if ctx == nil {
		return nil
	}
	return ctx.Value(larkClientKey)
}

// WithLarkChatID sets the Lark chat ID in context.
func WithLarkChatID(ctx context.Context, chatID string) context.Context {
	return context.WithValue(ctx, larkChatIDKey, chatID)
}

// LarkChatIDFromContext retrieves the Lark chat ID from context.
func LarkChatIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if chatID, ok := ctx.Value(larkChatIDKey).(string); ok {
		return chatID
	}
	return ""
}

// WithLarkMessageID sets the Lark message ID in context.
func WithLarkMessageID(ctx context.Context, messageID string) context.Context {
	return context.WithValue(ctx, larkMessageIDKey, messageID)
}

// LarkMessageIDFromContext retrieves the Lark message ID from context.
func LarkMessageIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if messageID, ok := ctx.Value(larkMessageIDKey).(string); ok {
		return messageID
	}
	return ""
}

// WithLarkOAuth stores the Lark OAuth service in context. Typed as interface{}
// to avoid importing the OAuth package at this shared boundary.
func WithLarkOAuth(ctx context.Context, svc interface{}) context.Context {
	return context.WithValue(ctx, larkOAuthKey, svc)
}

// LarkOAuthFromContext retrieves the Lark OAuth service from context.
func LarkOAuthFromContext(ctx context.Context) interface{} {
	if ctx == nil {
		return nil
	}
	return ctx.Value(larkOAuthKey)
}

// WithLarkTenantCalendarID stores the Lark tenant calendar ID in context.
func WithLarkTenantCalendarID(ctx context.Context, calendarID string) context.Context {
	return context.WithValue(ctx, larkTenantCalKey, calendarID)
}

// LarkTenantCalendarIDFromContext retrieves the Lark tenant calendar ID from context.
func LarkTenantCalendarIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if calendarID, ok := ctx.Value(larkTenantCalKey).(string); ok {
		return calendarID
	}
	return ""
}

// WithTimerManager sets the timer manager in context. Typed as interface{} to
// avoid importing the timer package in the shared package.
func WithTimerManager(ctx context.Context, mgr interface{}) context.Context {
	return context.WithValue(ctx, timerManagerKey, mgr)
}

// TimerManagerFromContext retrieves the timer manager from context.
func TimerManagerFromContext(ctx context.Context) interface{} {
	if ctx == nil {
		return nil
	}
	return ctx.Value(timerManagerKey)
}

// WithScheduler sets the scheduler in context. Typed as interface{} to avoid
// importing the scheduler package in the shared package.
func WithScheduler(ctx context.Context, sched interface{}) context.Context {
	return context.WithValue(ctx, schedulerKey, sched)
}

// SchedulerFromContext retrieves the scheduler from context.
func SchedulerFromContext(ctx context.Context) interface{} {
	if ctx == nil {
		return nil
	}
	return ctx.Value(schedulerKey)
}

// AutoUploadConfig controls automatic attachment uploads for local tools.
type AutoUploadConfig struct {
	Enabled   bool
	MaxBytes  int
	AllowExts []string
}

// WithAutoUploadConfig stores auto upload configuration in context.
func WithAutoUploadConfig(ctx context.Context, cfg AutoUploadConfig) context.Context {
	return context.WithValue(ctx, autoUploadKey, cfg)
}

// GetAutoUploadConfig retrieves auto upload configuration from context.
func GetAutoUploadConfig(ctx context.Context) AutoUploadConfig {
	if ctx == nil {
		return AutoUploadConfig{}
	}
	if cfg, ok := ctx.Value(autoUploadKey).(AutoUploadConfig); ok {
		return cfg
	}
	return AutoUploadConfig{}
}
