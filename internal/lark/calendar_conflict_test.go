package lark

import (
	"testing"
	"time"
)

// helper to build a CalendarEvent with minimal fields.
func makeEvent(id, summary, status string, start, end time.Time) CalendarEvent {
	return CalendarEvent{
		EventID:   id,
		Summary:   summary,
		Status:    status,
		StartTime: start,
		EndTime:   end,
	}
}

// base returns a fixed reference time (2026-02-01 09:00 UTC).
func base() time.Time {
	return time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
}

// --- DetectConflicts tests ---

func TestDetectConflicts_NoConflict(t *testing.T) {
	events := []CalendarEvent{
		makeEvent("1", "Morning standup", "confirmed", base(), base().Add(30*time.Minute)),
		makeEvent("2", "Lunch", "confirmed", base().Add(3*time.Hour), base().Add(4*time.Hour)),
	}

	// Proposed slot: 10:00–11:00, no overlap with 09:00–09:30 or 12:00–13:00.
	result := DetectConflicts(events, base().Add(1*time.Hour), base().Add(2*time.Hour))
	if result.HasConflict {
		t.Fatalf("expected no conflict, got %d conflicts", len(result.Conflicts))
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("expected empty Conflicts slice, got %d", len(result.Conflicts))
	}
}

func TestDetectConflicts_WithOverlap(t *testing.T) {
	events := []CalendarEvent{
		makeEvent("1", "Design review", "confirmed", base(), base().Add(1*time.Hour)),
		makeEvent("2", "Lunch", "confirmed", base().Add(3*time.Hour), base().Add(4*time.Hour)),
		makeEvent("3", "1:1", "confirmed", base().Add(30*time.Minute), base().Add(90*time.Minute)),
	}

	// Proposed slot: 09:15–10:15, overlaps with event 1 (09:00–10:00) and event 3 (09:30–10:30).
	result := DetectConflicts(events, base().Add(15*time.Minute), base().Add(75*time.Minute))
	if !result.HasConflict {
		t.Fatal("expected conflict, got none")
	}
	if len(result.Conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(result.Conflicts))
	}

	ids := map[string]bool{}
	for _, c := range result.Conflicts {
		ids[c.EventID] = true
	}
	if !ids["1"] || !ids["3"] {
		t.Fatalf("expected events 1 and 3 in conflicts, got %v", ids)
	}
}

func TestDetectConflicts_IgnoreCancelled(t *testing.T) {
	events := []CalendarEvent{
		makeEvent("1", "Cancelled sync", "cancelled", base(), base().Add(1*time.Hour)),
		makeEvent("2", "Active sync", "confirmed", base(), base().Add(1*time.Hour)),
	}

	// Proposed slot fully overlaps both, but only non-cancelled should appear.
	result := DetectConflicts(events, base(), base().Add(1*time.Hour))
	if !result.HasConflict {
		t.Fatal("expected conflict with the active event")
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result.Conflicts))
	}
	if result.Conflicts[0].EventID != "2" {
		t.Fatalf("expected conflict with event 2, got %s", result.Conflicts[0].EventID)
	}
}

// --- Edge cases for DetectConflicts ---

func TestDetectConflicts_AdjacentEvents(t *testing.T) {
	events := []CalendarEvent{
		// Ends exactly when proposed starts — no overlap.
		makeEvent("1", "Before", "confirmed", base(), base().Add(1*time.Hour)),
		// Starts exactly when proposed ends — no overlap.
		makeEvent("2", "After", "confirmed", base().Add(2*time.Hour), base().Add(3*time.Hour)),
	}

	result := DetectConflicts(events, base().Add(1*time.Hour), base().Add(2*time.Hour))
	if result.HasConflict {
		t.Fatalf("adjacent events should not conflict, got %d", len(result.Conflicts))
	}
}

func TestDetectConflicts_EmptyEvents(t *testing.T) {
	result := DetectConflicts(nil, base(), base().Add(1*time.Hour))
	if result.HasConflict {
		t.Fatal("empty event list should produce no conflict")
	}
}

// --- FindFreeSlots tests ---

func TestFindFreeSlots_Simple(t *testing.T) {
	// Range: 09:00–17:00 (8 hours).
	// Busy: 10:00–11:00, 14:00–15:00.
	// Expected free: 09:00–10:00, 11:00–14:00, 15:00–17:00.
	rangeStart := base()
	rangeEnd := base().Add(8 * time.Hour)

	events := []CalendarEvent{
		makeEvent("1", "Meeting A", "confirmed", base().Add(1*time.Hour), base().Add(2*time.Hour)),
		makeEvent("2", "Meeting B", "confirmed", base().Add(5*time.Hour), base().Add(6*time.Hour)),
	}

	slots := FindFreeSlots(events, rangeStart, rangeEnd, 30*time.Minute)
	if len(slots) != 3 {
		t.Fatalf("expected 3 free slots, got %d", len(slots))
	}

	// Slot 0: 09:00–10:00.
	assertSlot(t, slots[0], rangeStart, rangeStart.Add(1*time.Hour))
	// Slot 1: 11:00–14:00.
	assertSlot(t, slots[1], rangeStart.Add(2*time.Hour), rangeStart.Add(5*time.Hour))
	// Slot 2: 15:00–17:00.
	assertSlot(t, slots[2], rangeStart.Add(6*time.Hour), rangeEnd)
}

func TestFindFreeSlots_NoGaps(t *testing.T) {
	rangeStart := base()
	rangeEnd := base().Add(3 * time.Hour)

	// Back-to-back events covering the entire range.
	events := []CalendarEvent{
		makeEvent("1", "Block 1", "confirmed", rangeStart, rangeStart.Add(1*time.Hour)),
		makeEvent("2", "Block 2", "confirmed", rangeStart.Add(1*time.Hour), rangeStart.Add(2*time.Hour)),
		makeEvent("3", "Block 3", "confirmed", rangeStart.Add(2*time.Hour), rangeEnd),
	}

	slots := FindFreeSlots(events, rangeStart, rangeEnd, 1*time.Minute)
	if len(slots) != 0 {
		t.Fatalf("expected 0 free slots, got %d: %+v", len(slots), slots)
	}
}

func TestFindFreeSlots_OverlappingBusy(t *testing.T) {
	rangeStart := base()
	rangeEnd := base().Add(4 * time.Hour)

	// Two overlapping busy blocks that should merge into one 09:00–11:30 block.
	events := []CalendarEvent{
		makeEvent("1", "A", "confirmed", rangeStart, rangeStart.Add(2*time.Hour)),
		makeEvent("2", "B", "confirmed", rangeStart.Add(90*time.Minute), rangeStart.Add(150*time.Minute)),
	}

	slots := FindFreeSlots(events, rangeStart, rangeEnd, 30*time.Minute)
	// Free: 11:30–13:00 (90 min).
	if len(slots) != 1 {
		t.Fatalf("expected 1 free slot, got %d: %+v", len(slots), slots)
	}
	assertSlot(t, slots[0], rangeStart.Add(150*time.Minute), rangeEnd)
}

func TestFindFreeSlots_IgnoresCancelled(t *testing.T) {
	rangeStart := base()
	rangeEnd := base().Add(2 * time.Hour)

	events := []CalendarEvent{
		makeEvent("1", "Cancelled", "cancelled", rangeStart, rangeEnd),
	}

	slots := FindFreeSlots(events, rangeStart, rangeEnd, 30*time.Minute)
	// The cancelled event should be ignored, so the entire range is free.
	if len(slots) != 1 {
		t.Fatalf("expected 1 free slot (whole range), got %d", len(slots))
	}
	assertSlot(t, slots[0], rangeStart, rangeEnd)
}

func TestFindFreeSlots_DurationFilter(t *testing.T) {
	rangeStart := base()
	rangeEnd := base().Add(4 * time.Hour)

	// Creates a 15-min gap and a 2h55m gap.
	events := []CalendarEvent{
		makeEvent("1", "A", "confirmed", rangeStart, rangeStart.Add(50*time.Minute)),
		makeEvent("2", "B", "confirmed", rangeStart.Add(65*time.Minute), rangeEnd),
	}

	// Require 30-min slots — the 15-min gap should be excluded.
	slots := FindFreeSlots(events, rangeStart, rangeEnd, 30*time.Minute)
	if len(slots) != 0 {
		t.Fatalf("expected 0 free slots (gap too short), got %d: %+v", len(slots), slots)
	}

	// Require 10-min slots — the 15-min gap qualifies.
	slots = FindFreeSlots(events, rangeStart, rangeEnd, 10*time.Minute)
	if len(slots) != 1 {
		t.Fatalf("expected 1 free slot, got %d", len(slots))
	}
	assertSlot(t, slots[0], rangeStart.Add(50*time.Minute), rangeStart.Add(65*time.Minute))
}

// --- test helpers ---

func assertSlot(t *testing.T, slot TimeSlot, wantStart, wantEnd time.Time) {
	t.Helper()
	if !slot.Start.Equal(wantStart) {
		t.Errorf("slot start = %v, want %v", slot.Start, wantStart)
	}
	if !slot.End.Equal(wantEnd) {
		t.Errorf("slot end = %v, want %v", slot.End, wantEnd)
	}
}
