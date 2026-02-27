package shared

import (
	"context"

	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/backup"
	tmr "alex/internal/shared/timer"
)

// Context keys for tool dependencies
type toolContextKey string

const (
	ApproverKey      toolContextKey = "approver"
	BackupManagerKey toolContextKey = "backup_manager"
	ToolSessionIDKey toolContextKey = "tool_session_id"
	AutoApproveKey   toolContextKey = "auto_approve"
	larkClientKey    toolContextKey = "lark_client"
	larkMessengerKey toolContextKey = "lark_messenger"
	larkChatIDKey    toolContextKey = "lark_chat_id"
	larkMessageIDKey toolContextKey = "lark_message_id"
	larkOAuthKey     toolContextKey = "lark_oauth"
	larkTenantCalKey  toolContextKey = "lark_tenant_calendar_id"
	larkBaseDomainKey toolContextKey = "lark_base_domain"
	timerManagerKey  toolContextKey = "timer_manager"
	schedulerKey     toolContextKey = "scheduler"
	autoUploadKey    toolContextKey = "auto_upload_config"
)

type parentListenerKey struct{}

func contextValue[T any](ctx context.Context, key any) (T, bool) {
	var zero T
	if ctx == nil {
		return zero, false
	}

	value, ok := ctx.Value(key).(T)
	if !ok {
		return zero, false
	}

	return value, true
}

func contextValueOr[T any](ctx context.Context, key any, fallback T) T {
	if value, ok := contextValue[T](ctx, key); ok {
		return value
	}
	return fallback
}

func contextRawValue(ctx context.Context, key any) any {
	if ctx == nil {
		return nil
	}
	return ctx.Value(key)
}

// GetApproverFromContext retrieves the approver from context
func GetApproverFromContext(ctx context.Context) tools.Approver {
	return contextValueOr[tools.Approver](ctx, ApproverKey, nil)
}

// WithApprover sets the approver in context
func WithApprover(ctx context.Context, approver tools.Approver) context.Context {
	return context.WithValue(ctx, ApproverKey, approver)
}

// GetBackupManagerFromContext retrieves the backup manager from context
func GetBackupManagerFromContext(ctx context.Context) *backup.Manager {
	return contextValueOr[*backup.Manager](ctx, BackupManagerKey, nil)
}

// WithBackupManager sets the backup manager in context
func WithBackupManager(ctx context.Context, manager *backup.Manager) context.Context {
	return context.WithValue(ctx, BackupManagerKey, manager)
}

// GetToolSessionIDFromContext retrieves the session ID from context
func GetToolSessionIDFromContext(ctx context.Context) string {
	return contextValueOr[string](ctx, ToolSessionIDKey, "")
}

// WithToolSessionID sets the session ID in context
func WithToolSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, ToolSessionIDKey, sessionID)
}

// GetAutoApproveFromContext retrieves the auto-approve flag from context
func GetAutoApproveFromContext(ctx context.Context) bool {
	return contextValueOr[bool](ctx, AutoApproveKey, false)
}

// WithAutoApprove sets the auto-approve flag in context
func WithAutoApprove(ctx context.Context, autoApprove bool) context.Context {
	return context.WithValue(ctx, AutoApproveKey, autoApprove)
}

// GetParentListenerFromContext retrieves the parent listener (if any) for subtask event forwarding.
func GetParentListenerFromContext(ctx context.Context) agent.EventListener {
	return contextValueOr[agent.EventListener](ctx, parentListenerKey{}, nil)
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
	return contextRawValue(ctx, larkClientKey)
}

// LarkMessenger is a minimal messenger contract for sending chat text replies.
type LarkMessenger interface {
	SendMessage(ctx context.Context, chatID, msgType, content string) (string, error)
	ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error)
}

// WithLarkMessenger sets the Lark messenger in context.
func WithLarkMessenger(ctx context.Context, messenger LarkMessenger) context.Context {
	return context.WithValue(ctx, larkMessengerKey, messenger)
}

// LarkMessengerFromContext retrieves the Lark messenger from context.
func LarkMessengerFromContext(ctx context.Context) LarkMessenger {
	return contextValueOr[LarkMessenger](ctx, larkMessengerKey, nil)
}

// WithLarkChatID sets the Lark chat ID in context.
func WithLarkChatID(ctx context.Context, chatID string) context.Context {
	return context.WithValue(ctx, larkChatIDKey, chatID)
}

// LarkChatIDFromContext retrieves the Lark chat ID from context.
func LarkChatIDFromContext(ctx context.Context) string {
	return contextValueOr[string](ctx, larkChatIDKey, "")
}

// WithLarkMessageID sets the Lark message ID in context.
func WithLarkMessageID(ctx context.Context, messageID string) context.Context {
	return context.WithValue(ctx, larkMessageIDKey, messageID)
}

// LarkMessageIDFromContext retrieves the Lark message ID from context.
func LarkMessageIDFromContext(ctx context.Context) string {
	return contextValueOr[string](ctx, larkMessageIDKey, "")
}

// LarkOAuthService is the interface for Lark OAuth operations needed by tools.
type LarkOAuthService interface {
	UserAccessToken(ctx context.Context, openID string) (string, error)
	StartURL() string
}

// WithLarkOAuth stores the Lark OAuth service in context.
func WithLarkOAuth(ctx context.Context, svc LarkOAuthService) context.Context {
	return context.WithValue(ctx, larkOAuthKey, svc)
}

// LarkOAuthFromContext retrieves the Lark OAuth service from context.
func LarkOAuthFromContext(ctx context.Context) LarkOAuthService {
	return contextValueOr[LarkOAuthService](ctx, larkOAuthKey, nil)
}

// WithLarkBaseDomain stores the Lark API base domain in context.
func WithLarkBaseDomain(ctx context.Context, domain string) context.Context {
	return context.WithValue(ctx, larkBaseDomainKey, domain)
}

// LarkBaseDomainFromContext retrieves the Lark API base domain from context.
func LarkBaseDomainFromContext(ctx context.Context) string {
	return contextValueOr[string](ctx, larkBaseDomainKey, "")
}

// WithLarkTenantCalendarID stores the Lark tenant calendar ID in context.
func WithLarkTenantCalendarID(ctx context.Context, calendarID string) context.Context {
	return context.WithValue(ctx, larkTenantCalKey, calendarID)
}

// LarkTenantCalendarIDFromContext retrieves the Lark tenant calendar ID from context.
func LarkTenantCalendarIDFromContext(ctx context.Context) string {
	return contextValueOr[string](ctx, larkTenantCalKey, "")
}

// TimerManagerService is the interface for timer management needed by tools.
type TimerManagerService interface {
	Add(t *tmr.Timer) error
	Cancel(timerID string) error
	List(userID string) []tmr.Timer
	Get(timerID string) (tmr.Timer, bool)
}

// WithTimerManager sets the timer manager in context.
func WithTimerManager(ctx context.Context, mgr TimerManagerService) context.Context {
	return context.WithValue(ctx, timerManagerKey, mgr)
}

// TimerManagerFromContext retrieves the timer manager from context.
func TimerManagerFromContext(ctx context.Context) TimerManagerService {
	return contextValueOr[TimerManagerService](ctx, timerManagerKey, nil)
}

// WithScheduler sets the scheduler in context. Typed as interface{} to avoid
// importing the scheduler package in the shared package.
func WithScheduler(ctx context.Context, sched interface{}) context.Context {
	return context.WithValue(ctx, schedulerKey, sched)
}

// SchedulerFromContext retrieves the scheduler from context.
func SchedulerFromContext(ctx context.Context) interface{} {
	return contextRawValue(ctx, schedulerKey)
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
	return contextValueOr[AutoUploadConfig](ctx, autoUploadKey, AutoUploadConfig{})
}
