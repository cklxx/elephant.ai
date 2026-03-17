package lark

import (
	"testing"
	"time"

	"alex/internal/shared/logging"
)

// TestSnapshotWorker_TaskStartTimePreferredOverLastTouched verifies that when
// taskStartTime is set, Elapsed is calculated from it — NOT from lastTouched.
// This is the canonical correctness check for the fix in 03c9b995.
func TestSnapshotWorker_TaskStartTimePreferredOverLastTouched(t *testing.T) {
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	wantElapsed := 42 * time.Second

	gw := &Gateway{
		logger: logging.OrNop(nil),
		now:    func() time.Time { return base.Add(wantElapsed) },
	}
	gw.activeSlots.Store("oc_chat", &sessionSlot{
		phase:         slotRunning,
		taskDesc:      "deploy service",
		taskStartTime: base,
		lastTouched:   base.Add(-10 * time.Minute), // older; must NOT be used
	})

	snap := gw.snapshotWorker("oc_chat")
	if !snap.IsRunning() {
		t.Fatalf("expected slotRunning, got phase=%v", snap.Phase)
	}
	if snap.Elapsed != wantElapsed {
		t.Fatalf("elapsed: want %v (from taskStartTime), got %v (lastTouched fallback would give %v)",
			wantElapsed, snap.Elapsed, wantElapsed+10*time.Minute)
	}
}

// TestSnapshotWorker_ZeroTaskStartTime_FallsBackToLastTouched verifies the
// fallback path when taskStartTime has not yet been set (zero value).
func TestSnapshotWorker_ZeroTaskStartTime_FallsBackToLastTouched(t *testing.T) {
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	wantElapsed := 15 * time.Second

	gw := &Gateway{
		logger: logging.OrNop(nil),
		now:    func() time.Time { return base.Add(wantElapsed) },
	}
	gw.activeSlots.Store("oc_chat", &sessionSlot{
		phase:         slotRunning,
		taskDesc:      "analyze logs",
		taskStartTime: time.Time{}, // zero — triggers lastTouched fallback
		lastTouched:   base,
	})

	snap := gw.snapshotWorker("oc_chat")
	if !snap.IsRunning() {
		t.Fatalf("expected slotRunning, got phase=%v", snap.Phase)
	}
	if snap.Elapsed != wantElapsed {
		t.Fatalf("elapsed via lastTouched fallback: want %v, got %v", wantElapsed, snap.Elapsed)
	}
}
