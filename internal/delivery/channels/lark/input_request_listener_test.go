package lark

import (
	"strings"
	"testing"
	"time"

	agentports "alex/internal/domain/agent/ports/agent"
)

func TestIsSkipReply(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"skip", true},
		{"Skip", true},
		{"SKIP", true},
		{"跳过", true},
		{"pass", true},
		{"Pass", true},
		{"approve", false},
		{"1", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isSkipReply(tt.input)
		if got != tt.want {
			t.Errorf("isSkipReply(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseMultiNumberedReply(t *testing.T) {
	options := []string{"approve", "deny", "remember"}
	tests := []struct {
		input string
		want  int // expected length
	}{
		{"1,3", 2},
		{"1", 1},
		{"1,2,3", 3},
		{"hello", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseMultiNumberedReply(tt.input, options)
		if len(got) != tt.want {
			t.Errorf("parseMultiNumberedReply(%q) len = %d, want %d (got: %v)", tt.input, len(got), tt.want, got)
		}
	}
}

func TestBuildResponse_NumberedReply(t *testing.T) {
	relay := &pendingInputRelay{
		taskID:    "t1",
		requestID: "r1",
		options: []agentports.InputOption{
			{ID: "approve", Label: "同意"},
			{ID: "deny", Label: "拒绝"},
		},
	}

	resp := buildInputResponse(relay, "1")
	if resp.OptionID != "approve" {
		t.Errorf("expected OptionID=approve, got %q", resp.OptionID)
	}
	if !resp.Approved {
		t.Error("expected Approved=true")
	}

	resp = buildInputResponse(relay, "2")
	if resp.OptionID != "deny" {
		t.Errorf("expected OptionID=deny, got %q", resp.OptionID)
	}
}

func TestBuildResponse_DefaultPermission(t *testing.T) {
	relay := &pendingInputRelay{
		taskID:    "t1",
		requestID: "r1",
	}

	resp := buildInputResponse(relay, "1")
	if !resp.Approved {
		t.Error("1 should approve")
	}

	resp = buildInputResponse(relay, "2")
	if resp.Approved {
		t.Error("2 should deny")
	}

	resp = buildInputResponse(relay, "3")
	if !resp.Approved || resp.Text != "remember" {
		t.Error("3 should approve with remember")
	}
}

func TestBuildResponse_Skip(t *testing.T) {
	relay := &pendingInputRelay{
		taskID:    "t1",
		requestID: "r1",
	}

	resp := buildInputResponse(relay, "skip")
	if resp.Approved {
		t.Error("skip should not approve")
	}
	if resp.Text != "skipped" {
		t.Errorf("skip text should be 'skipped', got %q", resp.Text)
	}
}

func TestBuildResponse_FreeText(t *testing.T) {
	relay := &pendingInputRelay{
		taskID:    "t1",
		requestID: "r1",
	}

	resp := buildInputResponse(relay, "use the staging database instead")
	if !resp.Approved {
		t.Error("free text should default to approved")
	}
	if resp.Text != "use the staging database instead" {
		t.Errorf("unexpected text: %q", resp.Text)
	}
}

func TestFormatInputRequest(t *testing.T) {
	l := &inputRequestListener{}
	text := l.formatInputRequest("bg-abc", "claude_code", "Execute: npm test", "permission", nil)
	if text == "" {
		t.Error("formatted text should not be empty")
	}
	if !strings.Contains(text, "权限请求") {
		t.Error("should contain permission label")
	}
}

func TestPendingRelayQueue_FIFO(t *testing.T) {
	q := &pendingRelayQueue{}
	q.Push(&pendingInputRelay{taskID: "t1", requestID: "r1", createdAt: 1})
	q.Push(&pendingInputRelay{taskID: "t2", requestID: "r2", createdAt: 2})
	q.Push(&pendingInputRelay{taskID: "t3", requestID: "r3", createdAt: 3})

	r := q.PopOldest()
	if r == nil || r.taskID != "t1" {
		t.Fatalf("expected t1, got %v", r)
	}
	r = q.PopOldest()
	if r == nil || r.taskID != "t2" {
		t.Fatalf("expected t2, got %v", r)
	}
	r = q.PopOldest()
	if r == nil || r.taskID != "t3" {
		t.Fatalf("expected t3, got %v", r)
	}
	r = q.PopOldest()
	if r != nil {
		t.Fatalf("expected nil, got %v", r)
	}
}

func TestPendingRelayQueue_ReplaceSameTask(t *testing.T) {
	q := &pendingRelayQueue{}
	q.Push(&pendingInputRelay{taskID: "t1", requestID: "r1", createdAt: 1})
	q.Push(&pendingInputRelay{taskID: "t2", requestID: "r2", createdAt: 2})
	// t1 sends a new request — should replace the existing one
	q.Push(&pendingInputRelay{taskID: "t1", requestID: "r1-new", createdAt: 3})

	if q.Len() != 2 {
		t.Fatalf("expected 2 relays, got %d", q.Len())
	}

	r := q.PopOldest()
	if r == nil || r.taskID != "t1" || r.requestID != "r1-new" {
		t.Fatalf("expected t1/r1-new, got %v", r)
	}
	r = q.PopOldest()
	if r == nil || r.taskID != "t2" {
		t.Fatalf("expected t2, got %v", r)
	}
}

func TestPendingRelayQueue_PopOldestUnexpiredSkipsExpired(t *testing.T) {
	now := time.Now()
	q := &pendingRelayQueue{}
	q.Push(&pendingInputRelay{taskID: "expired", requestID: "r-exp", createdAt: 1, expiresAt: now.Add(-time.Second).UnixNano()})
	q.Push(&pendingInputRelay{taskID: "active", requestID: "r-ok", createdAt: 2, expiresAt: now.Add(time.Minute).UnixNano()})

	r := q.PopOldestUnexpired(now)
	if r == nil || r.taskID != "active" {
		t.Fatalf("expected active relay, got %v", r)
	}
	if q.Len() != 0 {
		t.Fatalf("expected queue empty after pop, got len=%d", q.Len())
	}
}

func TestPendingRelayQueue_PruneExpiredAndTrim(t *testing.T) {
	now := time.Now()
	q := &pendingRelayQueue{}
	q.Push(&pendingInputRelay{taskID: "a", requestID: "r1", createdAt: 1, expiresAt: now.Add(-time.Second).UnixNano()})
	q.Push(&pendingInputRelay{taskID: "b", requestID: "r2", createdAt: 2, expiresAt: now.Add(time.Minute).UnixNano()})
	q.Push(&pendingInputRelay{taskID: "c", requestID: "r3", createdAt: 3, expiresAt: now.Add(time.Minute).UnixNano()})
	q.Push(&pendingInputRelay{taskID: "d", requestID: "r4", createdAt: 4, expiresAt: now.Add(time.Minute).UnixNano()})

	if removed := q.PruneExpired(now); removed != 1 {
		t.Fatalf("expected one expired relay pruned, got %d", removed)
	}
	if q.Len() != 3 {
		t.Fatalf("expected len=3 after prune, got %d", q.Len())
	}
	if dropped := q.TrimToMax(2); dropped != 1 {
		t.Fatalf("expected one relay dropped by cap, got %d", dropped)
	}
	if q.Len() != 2 {
		t.Fatalf("expected len=2 after trim, got %d", q.Len())
	}
}
