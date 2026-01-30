package lark

import (
	"time"

	"alex/internal/channels"
)

// Config captures Lark gateway behavior.
type Config struct {
	channels.BaseConfig `yaml:",inline"`
	Enabled             bool
	AppID               string
	AppSecret           string
	BaseDomain          string
	SessionMode         string // "stable" (default) reuses chat session; "fresh" creates a new session per message.
	ReactEmoji          string // Emoji reaction sent immediately on message receipt (e.g. "SMILE"). Empty disables.
	ShowToolProgress    bool   // Show real-time tool progress in chat. Default false.
	AutoChatContext     bool   // Automatically fetch recent chat messages as context. Default false.
	AutoChatContextSize int    // Number of recent messages to fetch for auto chat context. Default 20.
	PlanReviewEnabled             bool
	PlanReviewRequireConfirmation bool
	PlanReviewPendingTTL          time.Duration
}
