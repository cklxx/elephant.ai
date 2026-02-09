package lark

import (
	"testing"

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
	l := &inputRequestListener{}

	resp := l.buildResponse(relay, "1")
	if resp.OptionID != "approve" {
		t.Errorf("expected OptionID=approve, got %q", resp.OptionID)
	}
	if !resp.Approved {
		t.Error("expected Approved=true")
	}

	resp = l.buildResponse(relay, "2")
	if resp.OptionID != "deny" {
		t.Errorf("expected OptionID=deny, got %q", resp.OptionID)
	}
}

func TestBuildResponse_DefaultPermission(t *testing.T) {
	relay := &pendingInputRelay{
		taskID:    "t1",
		requestID: "r1",
	}
	l := &inputRequestListener{}

	resp := l.buildResponse(relay, "1")
	if !resp.Approved {
		t.Error("1 should approve")
	}

	resp = l.buildResponse(relay, "2")
	if resp.Approved {
		t.Error("2 should deny")
	}

	resp = l.buildResponse(relay, "3")
	if !resp.Approved || resp.Text != "remember" {
		t.Error("3 should approve with remember")
	}
}

func TestBuildResponse_Skip(t *testing.T) {
	relay := &pendingInputRelay{
		taskID:    "t1",
		requestID: "r1",
	}
	l := &inputRequestListener{}

	resp := l.buildResponse(relay, "skip")
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
	l := &inputRequestListener{}

	resp := l.buildResponse(relay, "use the staging database instead")
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
	if !func() bool {
		for i := 0; i <= len(text)-len("权限请求"); i++ {
			if text[i:i+len("权限请求")] == "权限请求" {
				return true
			}
		}
		return false
	}() {
		t.Error("should contain permission label")
	}
}
