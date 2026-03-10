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

func TestSetPane(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	s.SetPane(42, 7)
	snap := s.Snapshot()
	if snap.PaneID != 42 {
		t.Errorf("PaneID = %d, want 42", snap.PaneID)
	}
	if snap.TabID != 7 {
		t.Errorf("TabID = %d, want 7", snap.TabID)
	}
}

func TestSetParentSession(t *testing.T) {
	s := session.New("child-1", session.MemberCodex, "goal", "/tmp")
	s.SetParentSession("leader-1")
	snap := s.Snapshot()
	if snap.ParentSessionID != "leader-1" {
		t.Errorf("ParentSessionID = %q, want leader-1", snap.ParentSessionID)
	}
}

func TestSetPoolPane(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	if s.Snapshot().PoolPane {
		t.Fatal("PoolPane should default to false")
	}
	s.SetPoolPane(true)
	if !s.Snapshot().PoolPane {
		t.Fatal("PoolPane should be true after SetPoolPane(true)")
	}
	s.SetPoolPane(false)
	if s.Snapshot().PoolPane {
		t.Fatal("PoolPane should be false after SetPoolPane(false)")
	}
}

func TestSetResult(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)
	_ = s.Transition(session.StateCompleted)

	s.SetResult("the answer is 42")
	snap := s.Snapshot()
	if snap.Answer != "the answer is 42" {
		t.Errorf("Answer = %q, want 'the answer is 42'", snap.Answer)
	}
}

func TestSetError(t *testing.T) {
	s := session.New("s1", session.MemberCodex, "goal", "/tmp")
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateFailed)

	s.SetError("connection refused")
	snap := s.Snapshot()
	if snap.ErrorMsg != "connection refused" {
		t.Errorf("ErrorMsg = %q, want 'connection refused'", snap.ErrorMsg)
	}
}

func TestSettersUpdateTimestamp(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	before := s.Snapshot().UpdatedAt

	time.Sleep(1 * time.Millisecond)
	s.SetPane(1, 1)
	after := s.Snapshot().UpdatedAt
	if !after.After(before) {
		t.Error("SetPane should advance UpdatedAt")
	}

	before = after
	time.Sleep(1 * time.Millisecond)
	s.SetParentSession("p1")
	after = s.Snapshot().UpdatedAt
	if !after.After(before) {
		t.Error("SetParentSession should advance UpdatedAt")
	}

	before = after
	time.Sleep(1 * time.Millisecond)
	s.SetPoolPane(true)
	after = s.Snapshot().UpdatedAt
	if !after.After(before) {
		t.Error("SetPoolPane should advance UpdatedAt")
	}

	before = after
	time.Sleep(1 * time.Millisecond)
	s.SetResult("answer")
	after = s.Snapshot().UpdatedAt
	if !after.After(before) {
		t.Error("SetResult should advance UpdatedAt")
	}

	before = after
	time.Sleep(1 * time.Millisecond)
	s.SetError("err")
	after = s.Snapshot().UpdatedAt
	if !after.After(before) {
		t.Error("SetError should advance UpdatedAt")
	}
}

func TestTransition_CancelledFromCreated(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	if err := s.Transition(session.StateCancelled); err != nil {
		t.Fatalf("should allow created→cancelled: %v", err)
	}
	if !s.IsTerminal() {
		t.Fatal("cancelled should be terminal")
	}
}

func TestTransition_StalledFromRunning(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)
	if err := s.Transition(session.StateStalled); err != nil {
		t.Fatalf("should allow running→stalled: %v", err)
	}
	// Can recover from stalled back to running.
	if err := s.Transition(session.StateRunning); err != nil {
		t.Fatalf("should allow stalled→running: %v", err)
	}
}

func TestIsStalled_TerminalNotStalled(t *testing.T) {
	s := session.New("s1", session.MemberCodex, "goal", "/tmp")
	_ = s.Transition(session.StateStarting)
	_ = s.Transition(session.StateRunning)
	_ = s.Transition(session.StateCompleted)
	// Terminal sessions are never stalled.
	if s.IsStalled(0) {
		t.Fatal("terminal session should not be stalled")
	}
}

func TestIsStalled_NoHeartbeatNoStart(t *testing.T) {
	s := session.New("s1", session.MemberClaudeCode, "goal", "/tmp")
	// Created state, no start, no heartbeat — not stalled.
	if s.IsStalled(0) {
		t.Fatal("session with no start should not be stalled")
	}
}
