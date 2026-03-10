package focustime

import (
	"sync"
	"time"
)

// Window represents a focus time window within a day (hour-based).
type Window struct {
	StartHour int // 0-23
	EndHour   int // 0-23, exclusive; wraps around midnight if Start > End
}

// Contains returns true if the given hour falls within the window.
func (w Window) Contains(hour int) bool {
	if w.StartHour <= w.EndHour {
		return hour >= w.StartHour && hour < w.EndHour
	}
	// Wraps around midnight, e.g. 22-8 means 22,23,0,1,...,7
	return hour >= w.StartHour || hour < w.EndHour
}

// Manager tracks focus time windows per user and provides a
// ShouldSuppress check for the attention gate.
type Manager struct {
	// globalWindow is the default quiet hours window applied to all users
	// unless overridden.
	globalWindow Window

	mu       sync.RWMutex
	perUser  map[string][]Window // userID → custom focus windows
}

// NewManager creates a Manager with global quiet hours derived from
// LeaderAttentionGateConfig.QuietHoursStart and QuietHoursEnd.
func NewManager(quietStart, quietEnd int) *Manager {
	return &Manager{
		globalWindow: Window{StartHour: quietStart, EndHour: quietEnd},
		perUser:      make(map[string][]Window),
	}
}

// SetUserWindows configures custom focus time windows for a specific user.
// These replace the global window for that user.
func (m *Manager) SetUserWindows(userID string, windows []Window) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(windows) == 0 {
		delete(m.perUser, userID)
		return
	}
	m.perUser[userID] = windows
}

// ClearUserWindows removes per-user overrides, reverting to global defaults.
func (m *Manager) ClearUserWindows(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.perUser, userID)
}

// ShouldSuppress returns true if the given user is currently in a focus
// time window at time now. Critical/P0 callers should bypass this check.
func (m *Manager) ShouldSuppress(userID string, now time.Time) bool {
	hour := now.Hour()

	m.mu.RLock()
	windows, hasCustom := m.perUser[userID]
	m.mu.RUnlock()

	if hasCustom {
		for _, w := range windows {
			if w.Contains(hour) {
				return true
			}
		}
		return false
	}

	return m.globalWindow.Contains(hour)
}

// GlobalWindow returns the currently configured global quiet hours window.
func (m *Manager) GlobalWindow() Window {
	return m.globalWindow
}
