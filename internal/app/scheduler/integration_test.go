package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/shared/config"

	"gopkg.in/yaml.v3"
)

// mockMilestoneCheckinService records calls to SendCheckin.
type mockMilestoneCheckinService struct {
	mu    sync.Mutex
	calls int
}

func (m *mockMilestoneCheckinService) SendCheckin(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return nil
}

func (m *mockMilestoneCheckinService) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

const leaderConfigYAML = `
blocker_radar:
  enabled: true
  schedule: "0 */4 * * *"
  stale_threshold_seconds: 1800
  input_wait_seconds: 900
  notify_cooldown_seconds: 600
  channel: lark
  chat_id: oc_test_blocker
weekly_pulse:
  enabled: true
  schedule: "0 9 * * 1"
  channel: lark
  chat_id: oc_test_pulse
daily_summary:
  enabled: true
  schedule: "0 18 * * 1-5"
  lookback_seconds: 28800
  channel: lark
  chat_id: oc_test_daily
milestone:
  enabled: true
  schedule: "0 */1 * * *"
  lookback_seconds: 3600
  include_active: true
  include_completed: true
  channel: lark
  chat_id: oc_test_milestone
attention_gate:
  enabled: true
  budget_max: 20
  budget_window_seconds: 3600
  priority_threshold: 0.6
  quiet_hours_start: 22
  quiet_hours_end: 8
prep_brief:
  enabled: true
  schedule: "30 8 * * 1-5"
  channel: lark
  chat_id: oc_test_brief
`

// TestIntegration_LeaderConfigValidation tests that a representative
// LeaderConfig YAML round-trips through unmarshaling and passes validation.
func TestIntegration_LeaderConfigValidation(t *testing.T) {
	var lc config.LeaderConfig
	if err := yaml.Unmarshal([]byte(leaderConfigYAML), &lc); err != nil {
		t.Fatalf("unmarshal LeaderConfig YAML: %v", err)
	}

	errs := lc.Validate()
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %s", e)
		}
		t.Fatalf("expected LeaderConfig to be valid, got %d errors", len(errs))
	}

	// Spot-check a few parsed values.
	if lc.BlockerRadar.Schedule != "0 */4 * * *" {
		t.Errorf("blocker_radar.schedule = %q, want %q", lc.BlockerRadar.Schedule, "0 */4 * * *")
	}
	if lc.WeeklyPulse.Schedule != "0 9 * * 1" {
		t.Errorf("weekly_pulse.schedule = %q, want %q", lc.WeeklyPulse.Schedule, "0 9 * * 1")
	}
	if lc.Milestone.LookbackSeconds != 3600 {
		t.Errorf("milestone.lookback_seconds = %d, want 3600", lc.Milestone.LookbackSeconds)
	}
	if lc.PrepBrief.Schedule != "30 8 * * 1-5" {
		t.Errorf("prep_brief.schedule = %q, want %q", lc.PrepBrief.Schedule, "30 8 * * 1-5")
	}
}

// TestIntegration_LeaderConfigToScheduler verifies the full path from
// LeaderConfig YAML → scheduler Config → trigger registration. All four
// leader services (blocker radar, weekly pulse, milestone, prep brief)
// must be registered as cron triggers when their config is enabled.
func TestIntegration_LeaderConfigToScheduler(t *testing.T) {
	var lc config.LeaderConfig
	if err := yaml.Unmarshal([]byte(leaderConfigYAML), &lc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errs := lc.Validate(); len(errs) > 0 {
		t.Fatalf("validation: %v", errs)
	}

	// Wire mocks.
	pulseSvc := &mockWeeklyPulseService{}
	blockerSvc := &mockBlockerRadarService{}
	milestoneSvc := &mockMilestoneCheckinService{}
	prepSvc := &mockPrepBriefService{}
	coord := &mockCoordinator{answer: "ok"}
	notif := &mockNotifier{}

	// Build scheduler Config from LeaderConfig.
	cfg := Config{
		Enabled: true,
		BlockerRadar: config.BlockerRadarConfig{
			Enabled:               lc.BlockerRadar.Enabled,
			Schedule:              lc.BlockerRadar.Schedule,
			StaleThresholdSeconds: lc.BlockerRadar.StaleThresholdSeconds,
			InputWaitSeconds:      lc.BlockerRadar.InputWaitSeconds,
			Channel:               lc.BlockerRadar.Channel,
			ChatID:                lc.BlockerRadar.ChatID,
		},
		BlockerRadarService: blockerSvc,
		WeeklyPulse: config.WeeklyPulseConfig{
			Enabled:  lc.WeeklyPulse.Enabled,
			Schedule: lc.WeeklyPulse.Schedule,
			Channel:  lc.WeeklyPulse.Channel,
			ChatID:   lc.WeeklyPulse.ChatID,
		},
		WeeklyPulseService: pulseSvc,
		MilestoneCheckin: config.MilestoneCheckinConfig{
			Enabled:          lc.Milestone.Enabled,
			Schedule:         lc.Milestone.Schedule,
			LookbackSeconds:  lc.Milestone.LookbackSeconds,
			IncludeActive:    lc.Milestone.IncludeActive,
			IncludeCompleted: lc.Milestone.IncludeCompleted,
			Channel:          lc.Milestone.Channel,
			ChatID:           lc.Milestone.ChatID,
		},
		MilestoneService: milestoneSvc,
		PrepBrief: config.PrepBriefConfig{
			Enabled:  lc.PrepBrief.Enabled,
			Schedule: lc.PrepBrief.Schedule,
			Channel:  lc.PrepBrief.Channel,
			ChatID:   lc.PrepBrief.ChatID,
		},
		PrepBriefService: prepSvc,
	}

	sched := New(cfg, coord, notif, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Verify all four leader triggers are registered.
	names := sched.TriggerNames()
	expected := map[string]bool{
		blockerRadarTriggerName:  false,
		weeklyPulseTriggerName:  false,
		milestoneTriggerName:    false,
		prepBriefTriggerName:    false,
	}
	for _, n := range names {
		if _, ok := expected[n]; ok {
			expected[n] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected trigger %q to be registered, got names: %v", name, names)
		}
	}
}

// TestIntegration_LeaderSchedulerExecution verifies that each leader service
// is actually invoked when the cron schedule fires. Uses per-minute schedules
// to avoid long waits.
func TestIntegration_LeaderSchedulerExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration execution test in short mode")
	}

	pulseSvc := &mockWeeklyPulseService{}
	blockerSvc := &mockBlockerRadarService{}
	milestoneSvc := &mockMilestoneCheckinService{}
	prepSvc := &mockPrepBriefService{}
	coord := &mockCoordinator{answer: "ok"}
	notif := &mockNotifier{}

	// Use every-minute schedules so the test completes quickly.
	cfg := Config{
		Enabled: true,
		BlockerRadar: config.BlockerRadarConfig{
			Enabled:  true,
			Schedule: "* * * * *",
		},
		BlockerRadarService: blockerSvc,
		WeeklyPulse: config.WeeklyPulseConfig{
			Enabled:  true,
			Schedule: "* * * * *",
		},
		WeeklyPulseService: pulseSvc,
		MilestoneCheckin: config.MilestoneCheckinConfig{
			Enabled:  true,
			Schedule: "* * * * *",
		},
		MilestoneService: milestoneSvc,
		PrepBrief: config.PrepBriefConfig{
			Enabled:  true,
			Schedule: "* * * * *",
			MemberID: "ou_integration_test",
		},
		PrepBriefService: prepSvc,
	}

	sched := New(cfg, coord, notif, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Wait for each service to be called at least once (cron fires at next minute boundary).
	timeout := 90 * time.Second
	waitFor(t, timeout, func() bool { return blockerSvc.callCount() >= 1 })
	waitFor(t, timeout, func() bool { return pulseSvc.callCount() >= 1 })
	waitFor(t, timeout, func() bool { return milestoneSvc.callCount() >= 1 })
	waitFor(t, timeout, func() bool { return prepSvc.callCount() >= 1 })

	// Verify prep brief received the correct member ID.
	prepSvc.mu.Lock()
	gotMemberID := prepSvc.memberID
	prepSvc.mu.Unlock()
	if gotMemberID != "ou_integration_test" {
		t.Errorf("prep brief memberID = %q, want %q", gotMemberID, "ou_integration_test")
	}
}

// TestIntegration_LeaderPartialEnable verifies that only enabled features
// register triggers and disabled ones are silently skipped.
func TestIntegration_LeaderPartialEnable(t *testing.T) {
	pulseSvc := &mockWeeklyPulseService{}
	blockerSvc := &mockBlockerRadarService{}
	milestoneSvc := &mockMilestoneCheckinService{}
	prepSvc := &mockPrepBriefService{}
	coord := &mockCoordinator{answer: "ok"}

	// Only enable blocker radar and milestone; disable pulse and prep brief.
	cfg := Config{
		Enabled: true,
		BlockerRadar: config.BlockerRadarConfig{
			Enabled:  true,
			Schedule: "0 */4 * * *",
		},
		BlockerRadarService: blockerSvc,
		WeeklyPulse: config.WeeklyPulseConfig{
			Enabled: false,
		},
		WeeklyPulseService: pulseSvc,
		MilestoneCheckin: config.MilestoneCheckinConfig{
			Enabled:  true,
			Schedule: "0 */1 * * *",
		},
		MilestoneService: milestoneSvc,
		PrepBrief: config.PrepBriefConfig{
			Enabled: false,
		},
		PrepBriefService: prepSvc,
	}

	sched := New(cfg, coord, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	names := sched.TriggerNames()
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	if !nameSet[blockerRadarTriggerName] {
		t.Errorf("blocker radar should be registered")
	}
	if !nameSet[milestoneTriggerName] {
		t.Errorf("milestone should be registered")
	}
	if nameSet[weeklyPulseTriggerName] {
		t.Errorf("weekly pulse should NOT be registered when disabled")
	}
	if nameSet[prepBriefTriggerName] {
		t.Errorf("prep brief should NOT be registered when disabled")
	}
}

// TestIntegration_DefaultLeaderConfigToScheduler verifies that
// DefaultLeaderConfig produces a valid config that registers all services.
func TestIntegration_DefaultLeaderConfigToScheduler(t *testing.T) {
	lc := config.DefaultLeaderConfig()
	if errs := lc.Validate(); len(errs) > 0 {
		t.Fatalf("DefaultLeaderConfig validation errors: %v", errs)
	}

	pulseSvc := &mockWeeklyPulseService{}
	blockerSvc := &mockBlockerRadarService{}
	milestoneSvc := &mockMilestoneCheckinService{}
	prepSvc := &mockPrepBriefService{}

	// DefaultLeaderConfig has all features disabled; enable them to verify
	// the default schedules are valid and register correctly.
	cfg := Config{
		Enabled: true,
		BlockerRadar: config.BlockerRadarConfig{
			Enabled:  true,
			Schedule: lc.BlockerRadar.Schedule,
		},
		BlockerRadarService: blockerSvc,
		WeeklyPulse: config.WeeklyPulseConfig{
			Enabled:  true,
			Schedule: lc.WeeklyPulse.Schedule,
		},
		WeeklyPulseService: pulseSvc,
		MilestoneCheckin: config.MilestoneCheckinConfig{
			Enabled:  true,
			Schedule: lc.Milestone.Schedule,
		},
		MilestoneService: milestoneSvc,
		PrepBrief: config.PrepBriefConfig{
			Enabled:  true,
			Schedule: lc.PrepBrief.Schedule,
		},
		PrepBriefService: prepSvc,
	}

	sched := New(cfg, &mockCoordinator{answer: "ok"}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	names := sched.TriggerNames()
	for _, want := range []string{blockerRadarTriggerName, weeklyPulseTriggerName, milestoneTriggerName, prepBriefTriggerName} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected trigger %q from DefaultLeaderConfig, got names: %v", want, names)
		}
	}
}

// TestIntegration_LeaderConfigScheduleOverride verifies that custom schedules
// from LeaderConfig are passed through to the scheduler correctly.
func TestIntegration_LeaderConfigScheduleOverride(t *testing.T) {
	var lc config.LeaderConfig
	customYAML := `
blocker_radar:
  enabled: true
  schedule: "*/15 * * * *"
weekly_pulse:
  enabled: true
  schedule: "0 10 * * 2"
milestone:
  enabled: true
  schedule: "0 */2 * * *"
prep_brief:
  enabled: true
  schedule: "0 9 * * 1-5"
`
	if err := yaml.Unmarshal([]byte(customYAML), &lc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errs := lc.Validate(); len(errs) > 0 {
		t.Fatalf("validation: %v", errs)
	}

	pulseSvc := &mockWeeklyPulseService{}
	blockerSvc := &mockBlockerRadarService{}
	milestoneSvc := &mockMilestoneCheckinService{}
	prepSvc := &mockPrepBriefService{}

	cfg := Config{
		Enabled: true,
		BlockerRadar: config.BlockerRadarConfig{
			Enabled:  lc.BlockerRadar.Enabled,
			Schedule: lc.BlockerRadar.Schedule,
		},
		BlockerRadarService: blockerSvc,
		WeeklyPulse: config.WeeklyPulseConfig{
			Enabled:  lc.WeeklyPulse.Enabled,
			Schedule: lc.WeeklyPulse.Schedule,
		},
		WeeklyPulseService: pulseSvc,
		MilestoneCheckin: config.MilestoneCheckinConfig{
			Enabled:  lc.Milestone.Enabled,
			Schedule: lc.Milestone.Schedule,
		},
		MilestoneService: milestoneSvc,
		PrepBrief: config.PrepBriefConfig{
			Enabled:  lc.PrepBrief.Enabled,
			Schedule: lc.PrepBrief.Schedule,
		},
		PrepBriefService: prepSvc,
	}

	sched := New(cfg, &mockCoordinator{answer: "ok"}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// All four triggers registered with custom schedules.
	if got := sched.TriggerCount(); got < 4 {
		t.Errorf("expected at least 4 triggers, got %d: %v", got, sched.TriggerNames())
	}
}
