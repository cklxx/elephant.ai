package config

// SchedulerConfig configures time-based proactive triggers.
type SchedulerConfig struct {
	Enabled                          bool                     `json:"enabled" yaml:"enabled"`
	Triggers                         []SchedulerTriggerConfig `json:"triggers" yaml:"triggers"`
	TriggerTimeoutSeconds            int                      `json:"trigger_timeout_seconds" yaml:"trigger_timeout_seconds"`
	ConcurrencyPolicy                string                   `json:"concurrency_policy" yaml:"concurrency_policy"`
	LeaderLockEnabled                bool                     `json:"leader_lock_enabled" yaml:"leader_lock_enabled"`
	LeaderLockName                   string                   `json:"leader_lock_name" yaml:"leader_lock_name"`
	LeaderLockAcquireIntervalSeconds int                      `json:"leader_lock_acquire_interval_seconds" yaml:"leader_lock_acquire_interval_seconds"`
	JobStorePath                     string                   `json:"job_store_path" yaml:"job_store_path"`
	CooldownSeconds                  int                      `json:"cooldown_seconds" yaml:"cooldown_seconds"`
	MaxConcurrent                    int                      `json:"max_concurrent" yaml:"max_concurrent"`
	RecoveryMaxRetries               int                      `json:"recovery_max_retries" yaml:"recovery_max_retries"`
	RecoveryBackoffSeconds           int                      `json:"recovery_backoff_seconds" yaml:"recovery_backoff_seconds"`
	CalendarReminder                 CalendarReminderConfig   `json:"calendar_reminder" yaml:"calendar_reminder"`
	Heartbeat                        HeartbeatConfig          `json:"heartbeat" yaml:"heartbeat"`
	MilestoneCheckin                 MilestoneCheckinConfig   `json:"milestone_checkin" yaml:"milestone_checkin"`
	WeeklyPulse                      WeeklyPulseConfig        `json:"weekly_pulse" yaml:"weekly_pulse"`
	BlockerRadar                     BlockerRadarConfig       `json:"blocker_radar" yaml:"blocker_radar"`
	PrepBrief                        PrepBriefConfig          `json:"prep_brief" yaml:"prep_brief"`
}

// MilestoneCheckinConfig configures periodic progress summary check-ins.
type MilestoneCheckinConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Schedule         string `json:"schedule" yaml:"schedule"`                     // cron expression, default "0 */1 * * *" (hourly)
	LookbackSeconds  int    `json:"lookback_seconds" yaml:"lookback_seconds"`     // default 3600 (1 hour)
	Channel          string `json:"channel" yaml:"channel"`                       // delivery channel: lark | moltbook
	ChatID           string `json:"chat_id" yaml:"chat_id"`
	IncludeActive    bool   `json:"include_active" yaml:"include_active"`         // include in-flight tasks (default true)
	IncludeCompleted bool   `json:"include_completed" yaml:"include_completed"`   // include recently finished tasks (default true)
}

// WeeklyPulseConfig configures the weekly pulse digest for the leader agent.
type WeeklyPulseConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Schedule string `json:"schedule" yaml:"schedule"` // cron expression, default "0 9 * * 1" (Monday 9am)
	Channel  string `json:"channel" yaml:"channel"`   // delivery channel: lark | moltbook
	ChatID   string `json:"chat_id" yaml:"chat_id"`
}

// BlockerRadarConfig configures proactive detection of stuck or blocked tasks.
type BlockerRadarConfig struct {
	Enabled               bool   `json:"enabled" yaml:"enabled"`
	Schedule              string `json:"schedule" yaml:"schedule"`                               // cron expression, default "*/10 * * * *" (every 10 min)
	StaleThresholdSeconds int    `json:"stale_threshold_seconds" yaml:"stale_threshold_seconds"` // default 1800 (30 min)
	InputWaitSeconds      int    `json:"input_wait_seconds" yaml:"input_wait_seconds"`           // default 900 (15 min)
	Channel               string `json:"channel" yaml:"channel"`                                 // delivery channel: lark | moltbook
	ChatID                string `json:"chat_id" yaml:"chat_id"`
}

// PrepBriefConfig configures scheduled 1:1 meeting prep briefs.
type PrepBriefConfig struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	Schedule        string `json:"schedule" yaml:"schedule"`                 // cron expression, default "30 8 * * 1-5" (weekdays 8:30am)
	LookbackSeconds int    `json:"lookback_seconds" yaml:"lookback_seconds"` // default 604800 (7 days)
	MemberID        string `json:"member_id" yaml:"member_id"`               // target member for the brief
	Channel         string `json:"channel" yaml:"channel"`
	ChatID          string `json:"chat_id" yaml:"chat_id"`
}

// CalendarReminderConfig configures the periodic calendar reminder trigger.
type CalendarReminderConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Schedule         string `json:"schedule" yaml:"schedule"`                     // cron expression, default "*/15 * * * *"
	LookAheadMinutes int    `json:"look_ahead_minutes" yaml:"look_ahead_minutes"` // default 120
	Channel          string `json:"channel" yaml:"channel"`                       // delivery channel: lark | moltbook
	UserID           string `json:"user_id" yaml:"user_id"`                       // for channel=lark, this must be Lark open_id (ou_*)
	ChatID           string `json:"chat_id" yaml:"chat_id"`
}

type SchedulerTriggerConfig struct {
	Name             string `json:"name" yaml:"name"`
	Schedule         string `json:"schedule" yaml:"schedule"`
	Task             string `json:"task" yaml:"task"`
	Channel          string `json:"channel" yaml:"channel"`
	UserID           string `json:"user_id" yaml:"user_id"` // for channel=lark, this must be Lark open_id (ou_*)
	ChatID           string `json:"chat_id" yaml:"chat_id"` // channel-specific chat ID for notifications
	ApprovalRequired bool   `json:"approval_required" yaml:"approval_required"`
	Risk             string `json:"risk" yaml:"risk"`
}

// HeartbeatConfig configures periodic heartbeat checks driven by scheduler or timers.
type HeartbeatConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Schedule         string `json:"schedule" yaml:"schedule"` // cron expression, default "*/30 * * * *"
	Task             string `json:"task" yaml:"task"`
	Channel          string `json:"channel" yaml:"channel"`
	UserID           string `json:"user_id" yaml:"user_id"` // for channel=lark, this must be Lark open_id (ou_*)
	ChatID           string `json:"chat_id" yaml:"chat_id"`
	QuietHours       [2]int `json:"quiet_hours" yaml:"quiet_hours"`
	WindowLookbackHr int    `json:"window_lookback_hours" yaml:"window_lookback_hours"`
}

// TimerConfig configures agent-initiated dynamic timers.
type TimerConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	StorePath          string `json:"store_path" yaml:"store_path"`                     // default: ~/.alex/timers
	MaxTimers          int    `json:"max_timers" yaml:"max_timers"`                     // default: 100
	TaskTimeoutSeconds int    `json:"task_timeout_seconds" yaml:"task_timeout_seconds"` // default: 900
	HeartbeatEnabled   bool   `json:"heartbeat_enabled" yaml:"heartbeat_enabled"`
	HeartbeatMinutes   int    `json:"heartbeat_minutes" yaml:"heartbeat_minutes"`
}

// AttentionConfig throttles proactive notifications.
type AttentionConfig struct {
	MaxDailyNotifications int     `json:"max_daily_notifications" yaml:"max_daily_notifications"`
	MinIntervalSeconds    int     `json:"min_interval_seconds" yaml:"min_interval_seconds"`
	QuietHours            [2]int  `json:"quiet_hours" yaml:"quiet_hours"`
	PriorityThreshold     float64 `json:"priority_threshold" yaml:"priority_threshold"`
}
