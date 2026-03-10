package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// LeaderConfig is the top-level configuration for all leader agent features.
type LeaderConfig struct {
	BlockerRadar  LeaderBlockerRadarConfig  `json:"blocker_radar" yaml:"blocker_radar"`
	WeeklyPulse   LeaderWeeklyPulseConfig   `json:"weekly_pulse" yaml:"weekly_pulse"`
	DailySummary  LeaderDailySummaryConfig  `json:"daily_summary" yaml:"daily_summary"`
	Milestone     LeaderMilestoneConfig     `json:"milestone" yaml:"milestone"`
	AttentionGate LeaderAttentionGateConfig `json:"attention_gate" yaml:"attention_gate"`
	PrepBrief     LeaderPrepBriefConfig     `json:"prep_brief" yaml:"prep_brief"`
}

// LeaderBlockerRadarConfig configures proactive detection of stuck tasks.
type LeaderBlockerRadarConfig struct {
	Enabled               bool   `json:"enabled" yaml:"enabled"`
	Schedule              string `json:"schedule" yaml:"schedule"`
	StaleThresholdSeconds int    `json:"stale_threshold_seconds" yaml:"stale_threshold_seconds"`
	InputWaitSeconds      int    `json:"input_wait_seconds" yaml:"input_wait_seconds"`
	NotifyCooldownSeconds int    `json:"notify_cooldown_seconds" yaml:"notify_cooldown_seconds"`
	Channel               string `json:"channel" yaml:"channel"`
	ChatID                string `json:"chat_id" yaml:"chat_id"`
}

// LeaderWeeklyPulseConfig configures the weekly team digest.
type LeaderWeeklyPulseConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Schedule string `json:"schedule" yaml:"schedule"`
	Channel  string `json:"channel" yaml:"channel"`
	ChatID   string `json:"chat_id" yaml:"chat_id"`
}

// LeaderDailySummaryConfig configures the daily activity digest.
type LeaderDailySummaryConfig struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	Schedule        string `json:"schedule" yaml:"schedule"`
	LookbackSeconds int    `json:"lookback_seconds" yaml:"lookback_seconds"`
	Channel         string `json:"channel" yaml:"channel"`
	ChatID          string `json:"chat_id" yaml:"chat_id"`
}

// LeaderMilestoneConfig configures periodic progress check-ins.
type LeaderMilestoneConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Schedule         string `json:"schedule" yaml:"schedule"`
	LookbackSeconds  int    `json:"lookback_seconds" yaml:"lookback_seconds"`
	IncludeActive    bool   `json:"include_active" yaml:"include_active"`
	IncludeCompleted bool   `json:"include_completed" yaml:"include_completed"`
	Channel          string `json:"channel" yaml:"channel"`
	ChatID           string `json:"chat_id" yaml:"chat_id"`
}

const (
	defaultAttentionGateSummarizeThreshold = 40
	defaultAttentionGateQueueThreshold     = 60
	defaultAttentionGateNotifyNowThreshold = 80
	defaultAttentionGateEscalateThreshold  = 90
)

// LeaderAttentionGateConfig configures notification throttling and attention routing.
type LeaderAttentionGateConfig struct {
	Enabled             bool `json:"enabled" yaml:"enabled"`
	BudgetMax           int  `json:"budget_max" yaml:"budget_max"`
	BudgetWindowSeconds int  `json:"budget_window_seconds" yaml:"budget_window_seconds"`
	QuietHoursStart     int  `json:"quiet_hours_start" yaml:"quiet_hours_start"`
	QuietHoursEnd       int  `json:"quiet_hours_end" yaml:"quiet_hours_end"`
	SummarizeThreshold  int  `json:"summarize_threshold" yaml:"summarize_threshold"`
	QueueThreshold      int  `json:"queue_threshold" yaml:"queue_threshold"`
	NotifyNowThreshold  int  `json:"notify_now_threshold" yaml:"notify_now_threshold"`
	EscalateThreshold   int  `json:"escalate_threshold" yaml:"escalate_threshold"`
}

// BudgetWindow returns the budget window as a time.Duration.
func (c LeaderAttentionGateConfig) BudgetWindow() time.Duration {
	return time.Duration(c.BudgetWindowSeconds) * time.Second
}

// RoutingThresholds returns the attention score thresholds with defaults applied.
func (c LeaderAttentionGateConfig) RoutingThresholds() (summarize, queue, notifyNow, escalate int) {
	summarize = c.SummarizeThreshold
	if summarize == 0 {
		summarize = defaultAttentionGateSummarizeThreshold
	}
	queue = c.QueueThreshold
	if queue == 0 {
		queue = defaultAttentionGateQueueThreshold
	}
	notifyNow = c.NotifyNowThreshold
	if notifyNow == 0 {
		notifyNow = defaultAttentionGateNotifyNowThreshold
	}
	escalate = c.EscalateThreshold
	if escalate == 0 {
		escalate = defaultAttentionGateEscalateThreshold
	}
	return summarize, queue, notifyNow, escalate
}

// LeaderPrepBriefConfig configures 1:1 meeting prep briefs.
type LeaderPrepBriefConfig struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	Schedule        string `json:"schedule" yaml:"schedule"`
	LookbackSeconds int    `json:"lookback_seconds" yaml:"lookback_seconds"`
	Channel         string `json:"channel" yaml:"channel"`
	ChatID          string `json:"chat_id" yaml:"chat_id"`
}

// DefaultLeaderConfig returns sensible defaults matching current hardcoded values.
func DefaultLeaderConfig() LeaderConfig {
	return LeaderConfig{
		BlockerRadar: LeaderBlockerRadarConfig{
			Enabled:               false,
			Schedule:              "*/10 * * * *", // every 10 min
			StaleThresholdSeconds: 1800,           // 30 min
			InputWaitSeconds:      900,            // 15 min
			NotifyCooldownSeconds: 86400,          // 24 hours
			Channel:               "lark",
		},
		WeeklyPulse: LeaderWeeklyPulseConfig{
			Enabled:  false,
			Schedule: "0 9 * * 1", // Monday 9am
		},
		DailySummary: LeaderDailySummaryConfig{
			Enabled:         false,
			Schedule:        "0 18 * * *", // 6pm daily
			LookbackSeconds: 86400,        // 24 hours
		},
		Milestone: LeaderMilestoneConfig{
			Enabled:          false,
			Schedule:         "0 */1 * * *", // hourly
			LookbackSeconds:  3600,          // 1 hour
			IncludeActive:    true,
			IncludeCompleted: true,
		},
		AttentionGate: LeaderAttentionGateConfig{
			Enabled:             false,
			BudgetMax:           0,   // unlimited
			BudgetWindowSeconds: 600, // 10 min
			QuietHoursStart:     22,
			QuietHoursEnd:       8,
			SummarizeThreshold:  defaultAttentionGateSummarizeThreshold,
			QueueThreshold:      defaultAttentionGateQueueThreshold,
			NotifyNowThreshold:  defaultAttentionGateNotifyNowThreshold,
			EscalateThreshold:   defaultAttentionGateEscalateThreshold,
		},
		PrepBrief: LeaderPrepBriefConfig{
			Enabled:         false,
			Schedule:        "0 9 * * 5", // Friday 9am
			LookbackSeconds: 604800,      // 7 days
			Channel:         "lark",
		},
	}
}

// cronParser matches the 5-field parser used by the scheduler.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// LeaderConfigError represents a single validation error.
type LeaderConfigError struct {
	Field   string
	Message string
}

func (e LeaderConfigError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks the LeaderConfig for invalid cron syntax, out-of-range
// thresholds, and conflicting settings. Returns nil if valid.
func (c LeaderConfig) Validate() []LeaderConfigError {
	var errs []LeaderConfigError

	add := func(field, msg string) {
		errs = append(errs, LeaderConfigError{Field: field, Message: msg})
	}

	// --- cron schedule validation ---
	schedules := []struct {
		field string
		value string
		on    bool
	}{
		{"blocker_radar.schedule", c.BlockerRadar.Schedule, c.BlockerRadar.Enabled},
		{"weekly_pulse.schedule", c.WeeklyPulse.Schedule, c.WeeklyPulse.Enabled},
		{"daily_summary.schedule", c.DailySummary.Schedule, c.DailySummary.Enabled},
		{"milestone.schedule", c.Milestone.Schedule, c.Milestone.Enabled},
		{"prep_brief.schedule", c.PrepBrief.Schedule, c.PrepBrief.Enabled},
	}
	for _, s := range schedules {
		sched := strings.TrimSpace(s.value)
		if sched == "" {
			if s.on {
				add(s.field, "schedule is required when feature is enabled")
			}
			continue
		}
		if _, err := cronParser.Parse(sched); err != nil {
			add(s.field, fmt.Sprintf("invalid cron expression: %v", err))
		}
	}

	// --- threshold range checks ---
	if c.BlockerRadar.StaleThresholdSeconds < 0 {
		add("blocker_radar.stale_threshold_seconds", "must be non-negative")
	}
	if c.BlockerRadar.InputWaitSeconds < 0 {
		add("blocker_radar.input_wait_seconds", "must be non-negative")
	}
	if c.BlockerRadar.NotifyCooldownSeconds < 0 {
		add("blocker_radar.notify_cooldown_seconds", "must be non-negative")
	}
	if c.BlockerRadar.Enabled && c.BlockerRadar.StaleThresholdSeconds > 0 &&
		c.BlockerRadar.InputWaitSeconds > c.BlockerRadar.StaleThresholdSeconds {
		add("blocker_radar.input_wait_seconds", "should not exceed stale_threshold_seconds")
	}

	if c.DailySummary.LookbackSeconds < 0 {
		add("daily_summary.lookback_seconds", "must be non-negative")
	}

	if c.Milestone.LookbackSeconds < 0 {
		add("milestone.lookback_seconds", "must be non-negative")
	}

	if c.PrepBrief.LookbackSeconds < 0 {
		add("prep_brief.lookback_seconds", "must be non-negative")
	}

	// --- attention gate thresholds ---
	if c.AttentionGate.BudgetMax < 0 {
		add("attention_gate.budget_max", "must be non-negative")
	}
	if c.AttentionGate.BudgetWindowSeconds < 0 {
		add("attention_gate.budget_window_seconds", "must be non-negative")
	}
	if c.AttentionGate.Enabled && c.AttentionGate.BudgetMax > 0 && c.AttentionGate.BudgetWindowSeconds == 0 {
		add("attention_gate.budget_window_seconds", "must be positive when budget_max is set")
	}
	if c.AttentionGate.QuietHoursStart < 0 || c.AttentionGate.QuietHoursStart > 23 {
		add("attention_gate.quiet_hours_start", "must be between 0 and 23")
	}
	if c.AttentionGate.QuietHoursEnd < 0 || c.AttentionGate.QuietHoursEnd > 23 {
		add("attention_gate.quiet_hours_end", "must be between 0 and 23")
	}
	if c.AttentionGate.SummarizeThreshold < 0 || c.AttentionGate.SummarizeThreshold > 100 {
		add("attention_gate.summarize_threshold", "must be between 0 and 100")
	}
	if c.AttentionGate.QueueThreshold < 0 || c.AttentionGate.QueueThreshold > 100 {
		add("attention_gate.queue_threshold", "must be between 0 and 100")
	}
	if c.AttentionGate.NotifyNowThreshold < 0 || c.AttentionGate.NotifyNowThreshold > 100 {
		add("attention_gate.notify_now_threshold", "must be between 0 and 100")
	}
	if c.AttentionGate.EscalateThreshold < 0 || c.AttentionGate.EscalateThreshold > 100 {
		add("attention_gate.escalate_threshold", "must be between 0 and 100")
	}
	summarizeThreshold, queueThreshold, notifyNowThreshold, escalateThreshold := c.AttentionGate.RoutingThresholds()
	if queueThreshold < summarizeThreshold {
		add("attention_gate.queue_threshold", "must be greater than or equal to summarize_threshold")
	}
	if notifyNowThreshold < queueThreshold {
		add("attention_gate.notify_now_threshold", "must be greater than or equal to queue_threshold")
	}
	if escalateThreshold < notifyNowThreshold {
		add("attention_gate.escalate_threshold", "must be greater than or equal to notify_now_threshold")
	}

	// --- conflicting settings ---
	if c.DailySummary.Enabled && c.WeeklyPulse.Enabled &&
		c.DailySummary.Schedule != "" && c.WeeklyPulse.Schedule != "" &&
		c.DailySummary.Schedule == c.WeeklyPulse.Schedule {
		add("daily_summary.schedule", "should not use the same schedule as weekly_pulse")
	}

	return errs
}
