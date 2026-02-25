package lark

import (
	"context"
	"time"
)

// ChatSessionBinding stores the active session for a Lark chat.
type ChatSessionBinding struct {
	Channel   string
	ChatID    string
	SessionID string
	UpdatedAt time.Time
}

// ChatSessionBindingStore persists chat->session bindings so a chat can keep
// using the same session across process restarts.
type ChatSessionBindingStore interface {
	EnsureSchema(ctx context.Context) error
	SaveBinding(ctx context.Context, binding ChatSessionBinding) error
	GetBinding(ctx context.Context, channel, chatID string) (ChatSessionBinding, bool, error)
	DeleteBinding(ctx context.Context, channel, chatID string) error
}
