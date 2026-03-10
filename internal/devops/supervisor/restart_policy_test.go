package supervisor

import (
	"testing"
	"time"
)

func TestRestartPolicyBasic(t *testing.T) {
	p := NewRestartPolicy(3, 10*time.Second, 5*time.Second)
	now := time.Now()

	// Initially no restarts recorded.
	if c := p.RestartCount("main", now); c != 0 {
		t.Errorf("RestartCount = %d, want 0", c)
	}

	// Record 3 restarts — should reach the threshold.
	for i := 0; i < 3; i++ {
		p.RecordRestart("main")
	}
	if c := p.RestartCount("main", now); c != 3 {
		t.Errorf("RestartCount = %d, want 3", c)
	}
}

func TestRestartPolicyWindowPruning(t *testing.T) {
	p := NewRestartPolicy(3, 1*time.Second, 5*time.Second)

	p.RecordRestart("main")
	p.RecordRestart("main")
	p.RecordRestart("main")

	now := time.Now()
	if c := p.RestartCount("main", now); c != 3 {
		t.Errorf("RestartCount = %d, want 3", c)
	}

	// After window expires, count should drop to 0.
	future := now.Add(2 * time.Second)
	if c := p.RestartCount("main", future); c != 0 {
		t.Errorf("RestartCount after window = %d, want 0", c)
	}
}

func TestRestartPolicyCooldown(t *testing.T) {
	p := NewRestartPolicy(3, 10*time.Second, 2*time.Second)
	now := time.Now()

	p.EnterCooldown("main")
	if !p.InCooldown("main", now) {
		t.Error("should be in cooldown")
	}

	// After cooldown duration
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
