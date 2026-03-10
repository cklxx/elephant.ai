//go:build integration

package scheduler

import (
	"context"
	"testing"
	"time"

	"alex/internal/shared/config"
)

// TestLeader_BlockerRadar_E2E verifies that the blocker radar leader job
// fires via cron, is registered in TriggerNames, and reports healthy via
// LeaderJobsHealth.
func TestLeader_BlockerRadar_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	blockerSvc := &mockBlockerRadarService{}
	notif := &mockNotifier{}
	coord := &mockCoordinator{answer: "ok"}

	cfg := Config{
		Enabled: true,
		BlockerRadar: config.BlockerRadarConfig{
			Enabled:  true,
			Schedule: "* * * * *", // every minute for fast test
		},
		BlockerRadarService: blockerSvc,
		ConcurrencyPolicy:   "skip",
	}

	sched := New(cfg, coord, notif, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer sched.Stop()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("scheduler start: %v", err)
	}

	// Wait for blocker radar to fire at least once.
	waitFor(t, 70*time.Second, func() bool {
		return blockerSvc.callCount() >= 1
	})

	// Verify trigger registered.
	names := sched.TriggerNames()
	found := false
	for _, n := range names {
		if n == blockerRadarTriggerName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s in trigger names %v", blockerRadarTriggerName, names)
	}

	// Verify health reports blocker_radar as registered and healthy.
	health := sched.LeaderJobsHealth()
	for _, h := range health {
		if h.Name == blockerRadarTriggerName && h.Registered {
			if !h.Healthy {
				t.Fatalf("blocker_radar registered but not healthy: %s", h.LastError)
			}
			return
		}
	}
	t.Fatal("blocker_radar not found in health status")
}

// TestLeader_WeeklyPulse_E2E verifies that the weekly pulse leader job
// fires via cron, is registered in TriggerNames, and reports healthy via
// LeaderJobsHealth.
func TestLeader_WeeklyPulse_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pulseSvc := &mockWeeklyPulseService{}
	notif := &mockNotifier{}
	coord := &mockCoordinator{answer: "ok"}

	cfg := Config{
		Enabled: true,
		WeeklyPulse: config.WeeklyPulseConfig{
			Enabled:  true,
			Schedule: "* * * * *", // every minute for fast test
		},
		WeeklyPulseService: pulseSvc,
		ConcurrencyPolicy:  "skip",
	}

	sched := New(cfg, coord, notif, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer sched.Stop()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("scheduler start: %v", err)
	}

	// Wait for weekly pulse to fire at least once.
	waitFor(t, 70*time.Second, func() bool {
		return pulseSvc.callCount() >= 1
	})

	// Verify trigger registered.
	names := sched.TriggerNames()
	found := false
	for _, n := range names {
		if n == weeklyPulseTriggerName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s in trigger names %v", weeklyPulseTriggerName, names)
	}

	// Verify health reports weekly_pulse as registered and healthy.
	health := sched.LeaderJobsHealth()
	for _, h := range health {
		if h.Name == weeklyPulseTriggerName && h.Registered {
			if !h.Healthy {
				t.Fatalf("weekly_pulse registered but not healthy: %s", h.LastError)
			}
			return
		}
	}
	t.Fatal("weekly_pulse not found in health status")
}

// TestLeader_PartialFeatureDisable verifies that when only blocker_radar is
// enabled, only its trigger is registered. Disabled features (weekly pulse,
// milestone, prep brief) must not appear in TriggerNames or health status.
func TestLeader_PartialFeatureDisable(t *testing.T) {
	blockerSvc := &mockBlockerRadarService{}
	coord := &mockCoordinator{answer: "ok"}

	cfg := Config{
		Enabled: true,
		BlockerRadar: config.BlockerRadarConfig{
			Enabled:  true,
			Schedule: "0 */4 * * *",
		},
		BlockerRadarService: blockerSvc,
		// All other leader features disabled (zero-value Enabled = false).
		WeeklyPulse:     config.WeeklyPulseConfig{Enabled: false},
		MilestoneCheckin: config.MilestoneCheckinConfig{Enabled: false},
		PrepBrief:       config.PrepBriefConfig{Enabled: false},
	}

	sched := New(cfg, coord, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer sched.Stop()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("scheduler start: %v", err)
	}

	// Build name set for quick lookup.
	names := sched.TriggerNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	// blocker_radar must be present.
	if !nameSet[blockerRadarTriggerName] {
		t.Errorf("expected %s in trigger names %v", blockerRadarTriggerName, names)
	}

	// Disabled features must NOT be present.
	for _, disabled := range []string{weeklyPulseTriggerName, milestoneTriggerName, prepBriefTriggerName} {
		if nameSet[disabled] {
			t.Errorf("trigger %s should NOT be registered when disabled, got names %v", disabled, names)
		}
	}

	// Health: only blocker_radar should be registered.
	health := sched.LeaderJobsHealth()
	for _, h := range health {
		switch h.Name {
		case blockerRadarTriggerName:
			if !h.Registered {
				t.Errorf("blocker_radar should be registered in health status")
			}
		default:
			if h.Registered {
				t.Errorf("leader job %s should NOT be registered when disabled", h.Name)
			}
		}
	}
}
