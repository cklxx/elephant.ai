package session_test

import (
	"testing"
	"time"

	"alex/internal/runtime/session"
)

func TestNew(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "write tests", "/tmp")
	if s.State != session.StateCreated {
		t.Fatalf("expected created, got %s", s.State)
	}
	if s.PaneID != -1 || s.TabID != -1 {
		t.Fatalf("expected unassigned pane/tab")
	}
}

func TestTransition_valid(t *testing.T) {
	s := session.New("s1", session.MemberCodex, "goal", "/tmp")

	transitions := []session.State{
		session.StateStarting,
		session.StateRunning,
		session.StateNeedsInput,
		session.StateRunning,
		session.StateCompleted,
	}
	for _, target := range transitions {
		if err := s.Transition(target); err != nil {
			t.Fatalf("unexpected error transitioning to %s: %v", target, err)
		}
	}
	if !s.IsTerminal() {
		t.Fatal("expected terminal after completed")
	}
}

func TestTransition_invalid(t *testing.T) {
	s := session.New("s1", session.MemberCodex, "goal", "/tmp")
	// Cannot go from created directly to completed.
	if err := s.Transition(session.StateCompleted); err == nil {
		t.Fatal("expected error for invalid transition created→completed")
	}
}

func TestIsStalled(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)

	// Just started — not stalled yet.
	if s.IsStalled(5 * time.Minute) {
		t.Fatal("should not be stalled immediately after start")
	}

	// Record a heartbeat, then check stall with very short threshold.
	s.RecordHeartbeat()
	if s.IsStalled(1 * time.Millisecond) {
		// After sleep it should stall.
	}
	time.Sleep(10 * time.Millisecond)
	if !s.IsStalled(1 * time.Millisecond) {
		t.Fatal("expected stalled after heartbeat timeout")
	}
}

func TestSnapshot(t *testing.T) {
	s := session.New("s1", session.MemberKimi, "goal", "/tmp")
	snap := s.Snapshot()
	if snap.ID != "s1" || snap.Member != session.MemberKimi {
		t.Fatalf("snapshot mismatch")
	}
}

func TestTerminalStateNoFurtherTransition(t *testing.T) {
	s := session.New("s1", session.MemberCodex, "goal", "/tmp")
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateFailed)
	if err := s.Transition(session.StateCancelled); err == nil {
		t.Fatal("expected error transitioning out of terminal state")
	}
}
