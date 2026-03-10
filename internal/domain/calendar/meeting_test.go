package calendar

import (
	"testing"
	"time"
)

func TestMeeting_Fields(t *testing.T) {
	now := time.Now()
	m := Meeting{
		ID:           "evt_123",
		Title:        "Weekly 1:1",
		Participants: []string{"alice", "bob"},
		StartTime:    now,
		EndTime:      now.Add(30 * time.Minute),
		Is1on1:       true,
	}

	if m.ID != "evt_123" {
		t.Errorf("ID = %q, want %q", m.ID, "evt_123")
	}
	if !m.Is1on1 {
		t.Error("Is1on1 should be true")
	}
	if len(m.Participants) != 2 {
		t.Errorf("Participants count = %d, want 2", len(m.Participants))
	}
	if m.EndTime.Before(m.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
}
