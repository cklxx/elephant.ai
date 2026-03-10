package leader

import (
	"testing"
	"time"

	"alex/internal/runtime/session"
)

func TestBuildHandoffContext_FullSession(t *testing.T) {
	started := time.Now().Add(-5 * time.Minute)
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{
			"sess-1": {
				ID:        "sess-1",
				Member:    session.MemberClaudeCode,
				Goal:      "fix the login bug",
				StartedAt: &started,
			},
		},
	}
	bus := newMockBus()
	a := New(rt, bus, nil)

	// Pre-set stall count.
	a.stallCounts["sess-1"] = 2

	ctx := a.buildHandoffContext("sess-1", "session stalled 3 times")
	if ctx.SessionID != "sess-1" {
		t.Errorf("expected session_id=sess-1, got %q", ctx.SessionID)
	}
	if ctx.Member != string(session.MemberClaudeCode) {
		t.Errorf("expected member=claude_code, got %q", ctx.Member)
	}
	if ctx.Goal != "fix the login bug" {
		t.Errorf("expected goal='fix the login bug', got %q", ctx.Goal)
	}
	if ctx.StallCount != 2 {
		t.Errorf("expected stall_count=2, got %d", ctx.StallCount)
	}
	if ctx.Elapsed == "" {
		t.Error("expected non-empty elapsed")
	}
	if ctx.Reason != "session stalled 3 times" {
		t.Errorf("expected reason='session stalled 3 times', got %q", ctx.Reason)
	}
	if ctx.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

func TestBuildHandoffContext_MissingSession(t *testing.T) {
	rt := &mockRuntime{sessions: map[string]session.SessionData{}}
	bus := newMockBus()
	a := New(rt, bus, nil)

	ctx := a.buildHandoffContext("missing-sess", "unknown failure")
	if ctx.SessionID != "missing-sess" {
		t.Errorf("expected session_id=missing-sess, got %q", ctx.SessionID)
	}
	if ctx.Member != "" {
		t.Errorf("expected empty member, got %q", ctx.Member)
	}
	if ctx.Goal != "" {
		t.Errorf("expected empty goal, got %q", ctx.Goal)
	}
	if ctx.Elapsed != "" {
		t.Errorf("expected empty elapsed, got %q", ctx.Elapsed)
	}
	if ctx.Reason != "unknown failure" {
		t.Errorf("expected reason='unknown failure', got %q", ctx.Reason)
	}
}

func TestRecommendAction_AfterMaxStalls(t *testing.T) {
	ctx := HandoffContext{StallCount: maxStallAttempts}
	action := recommendAction(ctx)
	if action != "abort" {
		t.Errorf("expected 'abort' after max stalls, got %q", action)
	}
}

func TestRecommendAction_BelowMax(t *testing.T) {
	ctx := HandoffContext{StallCount: 1}
	action := recommendAction(ctx)
	if action != "provide_input" {
		t.Errorf("expected 'provide_input' below max, got %q", action)
	}
}

func TestHandoffContext_PayloadRoundTrip(t *testing.T) {
	original := HandoffContext{
		SessionID:         "sess-42",
		Member:            "claude_code",
		Goal:              "deploy the service",
		Reason:            "too many stalls",
		StallCount:        3,
		Elapsed:           "5m30s",
		RecommendedAction: "abort",
		CreatedAt:         time.Now().Truncate(time.Millisecond),
	}
	payload := original.ToPayload()
	parsed := ParseHandoffContext(payload)

	if parsed.SessionID != original.SessionID {
		t.Errorf("session_id: %q != %q", parsed.SessionID, original.SessionID)
	}
	if parsed.Member != original.Member {
		t.Errorf("member: %q != %q", parsed.Member, original.Member)
	}
	if parsed.Goal != original.Goal {
		t.Errorf("goal: %q != %q", parsed.Goal, original.Goal)
	}
	if parsed.Reason != original.Reason {
		t.Errorf("reason: %q != %q", parsed.Reason, original.Reason)
	}
	if parsed.StallCount != original.StallCount {
		t.Errorf("stall_count: %d != %d", parsed.StallCount, original.StallCount)
	}
	if parsed.Elapsed != original.Elapsed {
		t.Errorf("elapsed: %q != %q", parsed.Elapsed, original.Elapsed)
	}
	if parsed.RecommendedAction != original.RecommendedAction {
		t.Errorf("recommended_action: %q != %q", parsed.RecommendedAction, original.RecommendedAction)
	}
}

func TestParseHandoffContext_NilPayload(t *testing.T) {
	ctx := ParseHandoffContext(nil)
	if ctx.SessionID != "" || ctx.Member != "" || ctx.Goal != "" {
		t.Error("expected zero-value context from nil payload")
	}
}

func TestEscalatePublishesStructuredPayload(t *testing.T) {
	started := time.Now().Add(-2 * time.Minute)
	rt := &mockRuntime{
		sessions: map[string]session.SessionData{
			"sess-1": {
				ID:        "sess-1",
				Member:    session.MemberClaudeCode,
				Goal:      "analyze data",
				StartedAt: &started,
			},
		},
	}
	bus := newMockBus()
	a := New(rt, bus, nil)
	a.stallCounts["sess-1"] = 3

	a.escalate("sess-1", "max stalls reached")

	events := bus.publishedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != "handoff_required" {
		t.Errorf("expected handoff_required, got %s", ev.Type)
	}
	// Verify structured payload.
	reason, _ := ev.Payload["reason"].(string)
	if reason != "max stalls reached" {
		t.Errorf("expected reason='max stalls reached', got %q", reason)
	}
	goal, _ := ev.Payload["goal"].(string)
	if goal != "analyze data" {
		t.Errorf("expected goal='analyze data', got %q", goal)
	}
	member, _ := ev.Payload["member"].(string)
	if member != "claude_code" {
		t.Errorf("expected member='claude_code', got %q", member)
	}
	stallCount, _ := ev.Payload["stall_count"].(int)
	if stallCount != 3 {
		t.Errorf("expected stall_count=3, got %d", stallCount)
	}
	recAction, _ := ev.Payload["recommended_action"].(string)
	if recAction != "abort" {
		t.Errorf("expected recommended_action='abort', got %q", recAction)
	}
}
