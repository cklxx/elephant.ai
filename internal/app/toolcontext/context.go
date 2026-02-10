package toolcontext

import (
	"context"

	agent "alex/internal/domain/agent/ports/agent"
	builtinshared "alex/internal/infra/tools/builtin/shared"
)

// AutoUploadConfig reuses the tool context payload expected by builtin tools.
type AutoUploadConfig = builtinshared.AutoUploadConfig

// LarkOAuthService is the OAuth contract consumed by Lark-aware tools.
type LarkOAuthService = builtinshared.LarkOAuthService

// WithParentListener attaches the parent listener for subtask forwarding.
func WithParentListener(ctx context.Context, listener agent.EventListener) context.Context {
	return builtinshared.WithParentListener(ctx, listener)
}

// WithLarkClient stores the Lark SDK client for tool access.
func WithLarkClient(ctx context.Context, client interface{}) context.Context {
	return builtinshared.WithLarkClient(ctx, client)
}

// WithLarkChatID stores the Lark chat ID for tool access.
func WithLarkChatID(ctx context.Context, chatID string) context.Context {
	return builtinshared.WithLarkChatID(ctx, chatID)
}

// WithLarkMessageID stores the Lark message ID for tool access.
func WithLarkMessageID(ctx context.Context, messageID string) context.Context {
	return builtinshared.WithLarkMessageID(ctx, messageID)
}

// WithLarkTenantCalendarID stores the tenant calendar ID for tool access.
func WithLarkTenantCalendarID(ctx context.Context, calendarID string) context.Context {
	return builtinshared.WithLarkTenantCalendarID(ctx, calendarID)
}

// WithLarkOAuth stores the user OAuth service for tool access.
func WithLarkOAuth(ctx context.Context, svc LarkOAuthService) context.Context {
	return builtinshared.WithLarkOAuth(ctx, svc)
}

// WithAutoUploadConfig stores auto-upload behavior flags for local tools.
func WithAutoUploadConfig(ctx context.Context, cfg AutoUploadConfig) context.Context {
	return builtinshared.WithAutoUploadConfig(ctx, cfg)
}

// GetAutoUploadConfig reads auto-upload behavior flags from context.
func GetAutoUploadConfig(ctx context.Context) AutoUploadConfig {
	return builtinshared.GetAutoUploadConfig(ctx)
}

// WithAllowLocalFetch marks the context as allowing local fetches.
func WithAllowLocalFetch(ctx context.Context) context.Context {
	return builtinshared.WithAllowLocalFetch(ctx)
}
