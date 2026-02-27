package telegram

import (
	"time"

	"alex/internal/delivery/channels"
)

// Config holds the resolved Telegram gateway configuration.
type Config struct {
	channels.BaseConfig
	Enabled       bool
	BotToken      string
	AllowedGroups []int64

	// State management
	ActiveSlotTTL        time.Duration
	ActiveSlotMaxEntries int
	StateCleanupInterval time.Duration

	// Persistence
	PersistenceMode            string
	PersistenceDir             string
	PersistenceRetention       time.Duration
	PersistenceMaxTasksPerChat int

	// Features
	MaxConcurrentTasks            int
	ShowToolProgress              bool
	SlowProgressSummaryEnabled    *bool
	SlowProgressSummaryDelay      time.Duration
	PlanReviewEnabled             bool
	PlanReviewRequireConfirmation bool
	PlanReviewPendingTTL          time.Duration
}
