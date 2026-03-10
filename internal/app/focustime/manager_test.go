package focustime

import (
	"sync"
	"testing"
	"time"
)

func timeAt(hour int) time.Time {
	return time.Date(2026, 3, 10, hour, 30, 0, 0, time.UTC)
}

// --- Window.Contains ---

func TestWindowContains_NormalRange(t *testing.T) {
	w := Window{StartHour: 9, EndHour: 11}
	cases := []struct {
		hour int
		want bool
	}{
		{8, false},
		{9, true},
		{10, true},
		{11, false},
		{12, false},
	}
	for _, tc := range cases {
		if got := w.Contains(tc.hour); got != tc.want {
			t.Errorf("Window{9,11}.Contains(%d) = %v, want %v", tc.hour, got, tc.want)
		}
	}
}

func TestWindowContains_WrapsMidnight(t *testing.T) {
	w := Window{StartHour: 22, EndHour: 8}
	cases := []struct {
		hour int
		want bool
	}{
		{21, false},
		{22, true},
		{23, true},
		{0, true},
		{7, true},
		{8, false},
		{12, false},
	}
	for _, tc := range cases {
		if got := w.Contains(tc.hour); got != tc.want {
			t.Errorf("Window{22,8}.Contains(%d) = %v, want %v", tc.hour, got, tc.want)
		}
	}
}

func TestWindowContains_SameStartEnd(t *testing.T) {
	// StartHour == EndHour with normal range means empty window.
	w := Window{StartHour: 10, EndHour: 10}
	for h := 0; h < 24; h++ {
		if w.Contains(h) {
			t.Errorf("Window{10,10}.Contains(%d) = true, want false (empty window)", h)
		}
	}
}

// --- Manager.ShouldSuppress ---

func TestShouldSuppress_GlobalWindow(t *testing.T) {
	m := NewManager(22, 8)

	if !m.ShouldSuppress("user1", timeAt(23)) {
		t.Error("expected suppression at 23:30 (global quiet 22-8)")
	}
	if !m.ShouldSuppress("user1", timeAt(3)) {
		t.Error("expected suppression at 03:30 (global quiet 22-8)")
	}
	if m.ShouldSuppress("user1", timeAt(12)) {
		t.Error("unexpected suppression at 12:30 (outside global quiet)")
	}
}

func TestShouldSuppress_PerUserOverride(t *testing.T) {
	m := NewManager(22, 8)

	// User has custom focus windows: 9-11am and 2-4pm.
	m.SetUserWindows("alice", []Window{
		{StartHour: 9, EndHour: 11},
		{StartHour: 14, EndHour: 16},
	})

	// Alice should be suppressed during her custom windows.
	if !m.ShouldSuppress("alice", timeAt(10)) {
		t.Error("expected suppression at 10:30 for alice (custom 9-11)")
	}
	if !m.ShouldSuppress("alice", timeAt(15)) {
		t.Error("expected suppression at 15:30 for alice (custom 14-16)")
	}
	// Alice should NOT be suppressed during global quiet hours (custom overrides global).
	if m.ShouldSuppress("alice", timeAt(23)) {
		t.Error("unexpected suppression at 23:30 for alice (custom windows don't include 22-8)")
	}
	// Non-custom user still gets global.
	if !m.ShouldSuppress("bob", timeAt(23)) {
		t.Error("expected suppression at 23:30 for bob (global quiet)")
	}
}

func TestShouldSuppress_ClearUserWindows(t *testing.T) {
	m := NewManager(22, 8)
	m.SetUserWindows("alice", []Window{{StartHour: 9, EndHour: 11}})

	// Clear reverts to global.
	m.ClearUserWindows("alice")
	if !m.ShouldSuppress("alice", timeAt(23)) {
		t.Error("after clear, expected global suppression at 23:30")
	}
	if m.ShouldSuppress("alice", timeAt(10)) {
		t.Error("after clear, unexpected suppression at 10:30 (global quiet is 22-8)")
	}
}

func TestShouldSuppress_SetEmptyWindowsReverts(t *testing.T) {
	m := NewManager(22, 8)
	m.SetUserWindows("alice", []Window{{StartHour: 9, EndHour: 11}})
	m.SetUserWindows("alice", nil) // empty → revert

	if !m.ShouldSuppress("alice", timeAt(0)) {
		t.Error("expected global suppression after setting empty windows")
	}
}

func TestShouldSuppress_NoQuietHours(t *testing.T) {
	// Same start and end → empty global window → never suppress.
	m := NewManager(0, 0)
	for h := 0; h < 24; h++ {
		if m.ShouldSuppress("user1", timeAt(h)) {
			t.Errorf("unexpected suppression at hour %d with no quiet hours", h)
		}
	}
}

func TestGlobalWindow(t *testing.T) {
	m := NewManager(22, 8)
	w := m.GlobalWindow()
	if w.StartHour != 22 || w.EndHour != 8 {
		t.Errorf("GlobalWindow = {%d,%d}, want {22,8}", w.StartHour, w.EndHour)
	}
}

// --- Concurrency ---

func TestShouldSuppress_ConcurrentAccess(t *testing.T) {
	m := NewManager(22, 8)
	var wg sync.WaitGroup

	// Concurrent reads and writes.
	for i := 0; i < 50; i++ {
		wg.Add(2)
		userID := "user"
		go func() {
			defer wg.Done()
			m.ShouldSuppress(userID, timeAt(10))
		}()
		go func() {
			defer wg.Done()
			m.SetUserWindows(userID, []Window{{StartHour: 9, EndHour: 11}})
		}()
	}
	wg.Wait()
}
