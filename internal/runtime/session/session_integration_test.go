//go:build integration

package session_test

import (
	"testing"
	"time"

	"alex/internal/runtime/session"
)

func TestSession_LifecycleFlow(t *testing.T) {
	s := session.New("sess-lifecycle", session.MemberClaudeCode, "ship feature", "/tmp/runtime")

	createdAt := s.CreatedAt
	if err := s.Transition(session.StateStarting); err != nil {
		t.Fatalf("Transition(starting): %v", err)
	}
	if err := s.Transition(session.StateRunning); err != nil {
		t.Fatalf("Transition(running): %v", err)
	}

	if s.StartedAt == nil {
		t.Fatal("StartedAt should be set after entering running")
	}
	startedAt := *s.StartedAt

	s.SetPane(41, 7)
	s.SetParentSession("leader-1")
	s.SetPoolPane(true)
	s.RecordHeartbeat()

	if err := s.Transition(session.StateNeedsInput); err != nil {
		t.Fatalf("Transition(needs_input): %v", err)
	}
	if err := s.Transition(session.StateRunning); err != nil {
		t.Fatalf("Transition(running from needs_input): %v", err)
	}

	s.SetResult("done")
	if err := s.Transition(session.StateCompleted); err != nil {
		t.Fatalf("Transition(completed): %v", err)
	}

	snap := s.Snapshot()
	if !snap.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt changed: got %v want %v", snap.CreatedAt, createdAt)
	}
	if snap.StartedAt == nil || !snap.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %v, want %v", snap.StartedAt, startedAt)
	}
	if snap.EndedAt == nil {
		t.Fatal("EndedAt should be set after completion")
	}
	if snap.State != session.StateCompleted {
		t.Fatalf("State = %q, want %q", snap.State, session.StateCompleted)
	}
	if snap.PaneID != 41 || snap.TabID != 7 {
		t.Fatalf("pane/tab = (%d,%d), want (41,7)", snap.PaneID, snap.TabID)
	}
	if !snap.PoolPane {
		t.Fatal("PoolPane should be true")
	}
	if snap.ParentSessionID != "leader-1" {
		t.Fatalf("ParentSessionID = %q, want leader-1", snap.ParentSessionID)
	}
	if snap.Answer != "done" {
		t.Fatalf("Answer = %q, want done", snap.Answer)
	}
	if snap.LastHeartbeat == nil {
		t.Fatal("LastHeartbeat should be recorded")
	}
	if !s.IsTerminal() {
		t.Fatal("session should be terminal after completion")
	}
	if s.IsStalled(0) {
		t.Fatal("terminal session should never be stalled")
	}
}

func TestSession_StallRecoveryFlow(t *testing.T) {
	s := session.New("sess-stalled", session.MemberCodex, "repair stalled job", "/tmp/runtime")

	if err := s.Transition(session.StateStarting); err != nil {
		t.Fatalf("Transition(starting): %v", err)
	}
	if err := s.Transition(session.StateRunning); err != nil {
		t.Fatalf("Transition(running): %v", err)
	}

	past := time.Now().Add(-3 * time.Second)
	s.StartedAt = &past

	if !s.IsStalled(2 * time.Second) {
		t.Fatal("session should be stalled before recovery heartbeat")
	}

	if err := s.Transition(session.StateStalled); err != nil {
		t.Fatalf("Transition(stalled): %v", err)
	}
	s.RecordHeartbeat()
	if err := s.Transition(session.StateRunning); err != nil {
		t.Fatalf("Transition(running from stalled): %v", err)
	}

	snap := s.Snapshot()
	if snap.State != session.StateRunning {
		t.Fatalf("State = %q, want %q", snap.State, session.StateRunning)
	}
	if snap.LastHeartbeat == nil {
		t.Fatal("LastHeartbeat should be set after recovery")
	}
	if s.IsStalled(time.Hour) {
		t.Fatal("session should not be stalled immediately after recovery")
	}
}
