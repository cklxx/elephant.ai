package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DefaultLeaderConfig
// ---------------------------------------------------------------------------

func TestDefaultLeaderConfig_Valid(t *testing.T) {
	cfg := DefaultLeaderConfig()
	errs := cfg.Validate()
	assert.Empty(t, errs, "default config should pass validation")
}

func TestDefaultLeaderConfig_AllDisabled(t *testing.T) {
	cfg := DefaultLeaderConfig()
	assert.False(t, cfg.BlockerRadar.Enabled)
	assert.False(t, cfg.WeeklyPulse.Enabled)
	assert.False(t, cfg.DailySummary.Enabled)
	assert.False(t, cfg.Milestone.Enabled)
	assert.False(t, cfg.AttentionGate.Enabled)
	assert.False(t, cfg.PrepBrief.Enabled)
}

func TestDefaultLeaderConfig_MatchesHardcodedDefaults(t *testing.T) {
	cfg := DefaultLeaderConfig()

	// blocker_radar
	assert.Equal(t, "*/10 * * * *", cfg.BlockerRadar.Schedule)
	assert.Equal(t, 1800, cfg.BlockerRadar.StaleThresholdSeconds)
	assert.Equal(t, 900, cfg.BlockerRadar.InputWaitSeconds)
	assert.Equal(t, 86400, cfg.BlockerRadar.NotifyCooldownSeconds)
	assert.Equal(t, "lark", cfg.BlockerRadar.Channel)

	// weekly_pulse
	assert.Equal(t, "0 9 * * 1", cfg.WeeklyPulse.Schedule)

	// daily_summary
	assert.Equal(t, "0 18 * * *", cfg.DailySummary.Schedule)
	assert.Equal(t, 86400, cfg.DailySummary.LookbackSeconds)

	// milestone
	assert.Equal(t, "0 */1 * * *", cfg.Milestone.Schedule)
	assert.Equal(t, 3600, cfg.Milestone.LookbackSeconds)
	assert.True(t, cfg.Milestone.IncludeActive)
	assert.True(t, cfg.Milestone.IncludeCompleted)

	// attention_gate
	assert.Equal(t, 0, cfg.AttentionGate.BudgetMax)
	assert.Equal(t, 600, cfg.AttentionGate.BudgetWindowSeconds)
	assert.Equal(t, 22, cfg.AttentionGate.QuietHoursStart)
	assert.Equal(t, 8, cfg.AttentionGate.QuietHoursEnd)
	assert.Equal(t, 40, cfg.AttentionGate.SummarizeThreshold)
	assert.Equal(t, 60, cfg.AttentionGate.QueueThreshold)
	assert.Equal(t, 80, cfg.AttentionGate.NotifyNowThreshold)
	assert.Equal(t, 90, cfg.AttentionGate.EscalateThreshold)

	// prep_brief
	assert.Equal(t, "0 9 * * 5", cfg.PrepBrief.Schedule)
	assert.Equal(t, 604800, cfg.PrepBrief.LookbackSeconds)
	assert.Equal(t, "lark", cfg.PrepBrief.Channel)
}

// ---------------------------------------------------------------------------
// BudgetWindow helper
// ---------------------------------------------------------------------------

func TestAttentionGateConfig_BudgetWindow(t *testing.T) {
	cfg := DefaultLeaderConfig().AttentionGate
	assert.Equal(t, 600_000_000_000, int(cfg.BudgetWindow().Nanoseconds())) // 10 min
}

func TestAttentionGateConfig_RoutingThresholds_Defaults(t *testing.T) {
	cfg := LeaderAttentionGateConfig{}
	summarize, queue, notifyNow, escalate := cfg.RoutingThresholds()
	assert.Equal(t, 40, summarize)
	assert.Equal(t, 60, queue)
	assert.Equal(t, 80, notifyNow)
	assert.Equal(t, 90, escalate)
}

// ---------------------------------------------------------------------------
// Cron schedule validation
// ---------------------------------------------------------------------------

func TestValidate_InvalidCronSchedule(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.Enabled = true
	cfg.BlockerRadar.Schedule = "not-a-cron"
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "blocker_radar.schedule", errs[0].Field)
	assert.Contains(t, errs[0].Message, "invalid cron expression")
}

func TestValidate_EmptyScheduleWhenEnabled(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.WeeklyPulse.Enabled = true
	cfg.WeeklyPulse.Schedule = ""
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "weekly_pulse.schedule", errs[0].Field)
	assert.Contains(t, errs[0].Message, "required when feature is enabled")
}

func TestValidate_EmptyScheduleWhenDisabled_OK(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.Milestone.Enabled = false
	cfg.Milestone.Schedule = ""
	errs := cfg.Validate()
	assert.Empty(t, errs)
}

func TestValidate_AllSchedulesChecked(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.Enabled = true
	cfg.BlockerRadar.Schedule = "bad"
	cfg.WeeklyPulse.Enabled = true
	cfg.WeeklyPulse.Schedule = "bad"
	cfg.DailySummary.Enabled = true
	cfg.DailySummary.Schedule = "bad"
	cfg.Milestone.Enabled = true
	cfg.Milestone.Schedule = "bad"
	cfg.PrepBrief.Enabled = true
	cfg.PrepBrief.Schedule = "bad"

	errs := cfg.Validate()
	// 5 invalid cron errors + 1 conflicting schedule (daily_summary == weekly_pulse == "bad")
	cronErrs := 0
	for _, e := range errs {
		if strings.Contains(e.Message, "invalid cron expression") {
			cronErrs++
		}
	}
	assert.Equal(t, 5, cronErrs, "each feature with bad cron should produce an error")
}

func TestValidate_ValidCronExpressions(t *testing.T) {
	cases := []string{
		"*/5 * * * *",
		"0 9 * * 1",
		"30 8 1 * *",
		"0 0 * * *",
		"0 */2 * * 1-5",
	}
	for _, expr := range cases {
		cfg := DefaultLeaderConfig()
		cfg.BlockerRadar.Enabled = true
		cfg.BlockerRadar.Schedule = expr
		errs := cfg.Validate()
		assert.Empty(t, errs, "cron %q should be valid", expr)
	}
}

// ---------------------------------------------------------------------------
// Threshold range checks
// ---------------------------------------------------------------------------

func TestValidate_NegativeStaleThreshold(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.StaleThresholdSeconds = -1
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "blocker_radar.stale_threshold_seconds", errs[0].Field)
}

func TestValidate_NegativeInputWait(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.InputWaitSeconds = -1
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "blocker_radar.input_wait_seconds", errs[0].Field)
}

func TestValidate_NegativeNotifyCooldown(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.NotifyCooldownSeconds = -1
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "blocker_radar.notify_cooldown_seconds", errs[0].Field)
}

func TestValidate_InputWaitExceedsStale(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.Enabled = true
	cfg.BlockerRadar.StaleThresholdSeconds = 300
	cfg.BlockerRadar.InputWaitSeconds = 600
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "blocker_radar.input_wait_seconds" {
			found = true
			assert.Contains(t, e.Message, "should not exceed stale_threshold_seconds")
		}
	}
	assert.True(t, found, "expected input_wait > stale_threshold error")
}

func TestValidate_InputWaitExceedsStale_IgnoredWhenDisabled(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.Enabled = false
	cfg.BlockerRadar.StaleThresholdSeconds = 300
	cfg.BlockerRadar.InputWaitSeconds = 600
	errs := cfg.Validate()
	assert.Empty(t, errs)
}

func TestValidate_NegativeLookbackSeconds(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.DailySummary.LookbackSeconds = -1
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "daily_summary.lookback_seconds", errs[0].Field)

	cfg2 := DefaultLeaderConfig()
	cfg2.Milestone.LookbackSeconds = -1
	errs2 := cfg2.Validate()
	require.NotEmpty(t, errs2)
	assert.Equal(t, "milestone.lookback_seconds", errs2[0].Field)

	cfg3 := DefaultLeaderConfig()
	cfg3.PrepBrief.LookbackSeconds = -1
	errs3 := cfg3.Validate()
	require.NotEmpty(t, errs3)
	assert.Equal(t, "prep_brief.lookback_seconds", errs3[0].Field)
}

// ---------------------------------------------------------------------------
// Attention gate validation
// ---------------------------------------------------------------------------

func TestValidate_NegativeBudgetMax(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.AttentionGate.BudgetMax = -1
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "attention_gate.budget_max", errs[0].Field)
}

func TestValidate_BudgetMaxWithoutWindow(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.AttentionGate.Enabled = true
	cfg.AttentionGate.BudgetMax = 10
	cfg.AttentionGate.BudgetWindowSeconds = 0
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "attention_gate.budget_window_seconds" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestValidate_BudgetMaxWithoutWindow_IgnoredWhenDisabled(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.AttentionGate.Enabled = false
	cfg.AttentionGate.BudgetMax = 10
	cfg.AttentionGate.BudgetWindowSeconds = 0
	errs := cfg.Validate()
	assert.Empty(t, errs)
}

func TestValidate_AttentionRoutingThresholdOutOfRange(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.AttentionGate.SummarizeThreshold = 101
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "attention_gate.summarize_threshold", errs[0].Field)

	cfg2 := DefaultLeaderConfig()
	cfg2.AttentionGate.EscalateThreshold = -1
	errs2 := cfg2.Validate()
	require.NotEmpty(t, errs2)
	assert.Equal(t, "attention_gate.escalate_threshold", errs2[0].Field)
}

func TestValidate_AttentionRoutingThresholdBoundary(t *testing.T) {
	for _, v := range []int{0, 40, 100} {
		cfg := DefaultLeaderConfig()
		cfg.AttentionGate.SummarizeThreshold = v
		cfg.AttentionGate.QueueThreshold = v
		cfg.AttentionGate.NotifyNowThreshold = v
		cfg.AttentionGate.EscalateThreshold = v
		errs := cfg.Validate()
		assert.Empty(t, errs, "attention thresholds=%v should be valid", v)
	}
}

func TestValidate_AttentionRoutingThresholdOrder(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.AttentionGate.SummarizeThreshold = 50
	cfg.AttentionGate.QueueThreshold = 40
	cfg.AttentionGate.NotifyNowThreshold = 30
	cfg.AttentionGate.EscalateThreshold = 20

	errs := cfg.Validate()
	require.NotEmpty(t, errs)

	fields := map[string]bool{}
	for _, err := range errs {
		fields[err.Field] = true
	}

	assert.True(t, fields["attention_gate.queue_threshold"])
	assert.True(t, fields["attention_gate.notify_now_threshold"])
	assert.True(t, fields["attention_gate.escalate_threshold"])
}

func TestValidate_QuietHoursOutOfRange(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.AttentionGate.QuietHoursStart = 25
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	assert.Equal(t, "attention_gate.quiet_hours_start", errs[0].Field)

	cfg2 := DefaultLeaderConfig()
	cfg2.AttentionGate.QuietHoursEnd = -1
	errs2 := cfg2.Validate()
	require.NotEmpty(t, errs2)
	assert.Equal(t, "attention_gate.quiet_hours_end", errs2[0].Field)
}

// ---------------------------------------------------------------------------
// Conflicting settings
// ---------------------------------------------------------------------------

func TestValidate_ConflictingSchedules(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.DailySummary.Enabled = true
	cfg.DailySummary.Schedule = "0 9 * * 1"
	cfg.WeeklyPulse.Enabled = true
	cfg.WeeklyPulse.Schedule = "0 9 * * 1"
	errs := cfg.Validate()
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "daily_summary.schedule" && e.Message == "should not use the same schedule as weekly_pulse" {
			found = true
		}
	}
	assert.True(t, found, "expected conflicting schedule error")
}

func TestValidate_SameScheduleDifferentFeatures_OK(t *testing.T) {
	// Same schedule is fine between non-conflicting features.
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.Enabled = true
	cfg.BlockerRadar.Schedule = "0 9 * * 1"
	cfg.PrepBrief.Enabled = true
	cfg.PrepBrief.Schedule = "0 9 * * 1"
	errs := cfg.Validate()
	assert.Empty(t, errs)
}

// ---------------------------------------------------------------------------
// LeaderConfigError
// ---------------------------------------------------------------------------

func TestLeaderConfigError_ErrorString(t *testing.T) {
	e := LeaderConfigError{Field: "foo.bar", Message: "is required"}
	assert.Equal(t, "foo.bar: is required", e.Error())
}

// ---------------------------------------------------------------------------
// Full enabled config
// ---------------------------------------------------------------------------

func TestValidate_AllEnabled_ValidConfig(t *testing.T) {
	cfg := DefaultLeaderConfig()
	cfg.BlockerRadar.Enabled = true
	cfg.WeeklyPulse.Enabled = true
	cfg.DailySummary.Enabled = true
	cfg.Milestone.Enabled = true
	cfg.AttentionGate.Enabled = true
	cfg.PrepBrief.Enabled = true
	errs := cfg.Validate()
	assert.Empty(t, errs, "all-enabled default config should pass validation")
}
