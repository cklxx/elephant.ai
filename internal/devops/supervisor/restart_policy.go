package supervisor

import (
	"sync"
	"time"
)

// RestartPolicy tracks restart history and enforces storm detection.
type RestartPolicy struct {
	MaxInWindow      int
	WindowDuration   time.Duration
	CooldownDuration time.Duration

	history       map[string][]time.Time // component -> restart timestamps
	cooldownUntil map[string]time.Time
	mu            sync.Mutex
}

// NewRestartPolicy creates a new restart policy with the given parameters.
func NewRestartPolicy(maxInWindow int, window, cooldown time.Duration) *RestartPolicy {
	return &RestartPolicy{
		MaxInWindow:      maxInWindow,
		WindowDuration:   window,
		CooldownDuration: cooldown,
		history:          make(map[string][]time.Time),
		cooldownUntil:    make(map[string]time.Time),
	}
}

// RecordRestart records a restart attempt for a component. Returns the count
// of restarts within the current window.
func (p *RestartPolicy) RecordRestart(component string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	p.pruneHistory(component, now)
	p.history[component] = append(p.history[component], now)
	return len(p.history[component])
}

// ShouldRestart returns true if the component can be restarted without
// exceeding the storm threshold.
func (p *RestartPolicy) ShouldRestart(component string, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check cooldown
	if until, ok := p.cooldownUntil[component]; ok && now.Before(until) {
		return false
	}

	// Check global cooldown (empty key)
	if until, ok := p.cooldownUntil[""]; ok && now.Before(until) {
		return false
	}

	p.pruneHistory(component, now)
	return len(p.history[component]) < p.MaxInWindow
}

// InCooldown returns true if the component (or global) is in cooldown.
func (p *RestartPolicy) InCooldown(component string, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if until, ok := p.cooldownUntil[component]; ok && now.Before(until) {
		return true
	}
	if until, ok := p.cooldownUntil[""]; ok && now.Before(until) {
		return true
	}
	return false
}

// EnterCooldown puts a component (or global with empty string) into cooldown.
func (p *RestartPolicy) EnterCooldown(component string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cooldownUntil[component] = time.Now().Add(p.CooldownDuration)
}

// RestartCount returns the number of restarts for a component in the current window.
func (p *RestartPolicy) RestartCount(component string, now time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pruneHistory(component, now)
	return len(p.history[component])
}

// TotalRestartCount returns the total restart count across all components.
func (p *RestartPolicy) TotalRestartCount(now time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for comp := range p.history {
		p.pruneHistory(comp, now)
		total += len(p.history[comp])
	}
	return total
}

// Reset clears all history for a component.
func (p *RestartPolicy) Reset(component string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.history, component)
	delete(p.cooldownUntil, component)
}

func (p *RestartPolicy) pruneHistory(component string, now time.Time) {
	cutoff := now.Add(-p.WindowDuration)
	entries := p.history[component]
	pruned := entries[:0]
	for _, t := range entries {
		if !t.Before(cutoff) {
			pruned = append(pruned, t)
		}
	}
	p.history[component] = pruned
}
