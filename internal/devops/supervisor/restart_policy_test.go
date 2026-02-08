package supervisor

import (
	"testing"
	"time"
)

func TestRestartPolicyBasic(t *testing.T) {
	p := NewRestartPolicy(3, 10*time.Second, 5*time.Second)
	now := time.Now()

	// Should allow restarts initially
	if !p.ShouldRestart("main", now) {
		t.Error("should allow restart initially")
	}

	// Record 3 restarts
	for i := 0; i < 3; i++ {
		p.RecordRestart("main")
	}

	// Should still allow (count == max)
	if p.ShouldRestart("main", now) {
		t.Error("should deny restart at max")
	}
}

func TestRestartPolicyWindowPruning(t *testing.T) {
	p := NewRestartPolicy(3, 1*time.Second, 5*time.Second)

	// Record restarts
	p.RecordRestart("main")
	p.RecordRestart("main")
	p.RecordRestart("main")

	now := time.Now()
	if p.ShouldRestart("main", now) {
		t.Error("should deny at max")
	}

	// After window expires, should allow again
	future := now.Add(2 * time.Second)
	if !p.ShouldRestart("main", future) {
		t.Error("should allow after window expiry")
	}
}

func TestRestartPolicyCooldown(t *testing.T) {
	p := NewRestartPolicy(3, 10*time.Second, 2*time.Second)
	now := time.Now()

	p.EnterCooldown("main")
	if !p.InCooldown("main", now) {
		t.Error("should be in cooldown")
	}
	if p.ShouldRestart("main", now) {
		t.Error("should deny during cooldown")
	}

	// After cooldown
	future := now.Add(3 * time.Second)
	if p.InCooldown("main", future) {
		t.Error("should not be in cooldown after duration")
	}
}

func TestRestartPolicyGlobalCooldown(t *testing.T) {
	p := NewRestartPolicy(3, 10*time.Second, 2*time.Second)
	now := time.Now()

	p.EnterCooldown("") // global
	if !p.InCooldown("main", now) {
		t.Error("global cooldown should affect all components")
	}
}

func TestRestartPolicyTotalCount(t *testing.T) {
	p := NewRestartPolicy(10, 10*time.Second, 5*time.Second)
	now := time.Now()

	p.RecordRestart("main")
	p.RecordRestart("main")
	p.RecordRestart("test")

	total := p.TotalRestartCount(now)
	if total != 3 {
		t.Errorf("TotalRestartCount = %d, want 3", total)
	}
}

func TestRestartPolicyReset(t *testing.T) {
	p := NewRestartPolicy(3, 10*time.Second, 5*time.Second)

	p.RecordRestart("main")
	p.RecordRestart("main")
	p.EnterCooldown("main")

	p.Reset("main")

	now := time.Now()
	if !p.ShouldRestart("main", now) {
		t.Error("should allow restart after reset")
	}
	if p.InCooldown("main", now) {
		t.Error("should not be in cooldown after reset")
	}
}
