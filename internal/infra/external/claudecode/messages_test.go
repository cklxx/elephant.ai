package claudecode

import (
	"testing"
)

func TestParseStreamMessageResult(t *testing.T) {
	line := []byte(`{"type":"result","output":"done","usage":{"input_tokens":10,"output_tokens":5},"cost":0.02}`)
	msg, err := ParseStreamMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != "result" {
		t.Fatalf("expected type result, got %s", msg.Type)
	}
	if msg.ExtractText() != "done" {
		t.Fatalf("expected output text 'done', got %q", msg.ExtractText())
	}
	tokens, cost := msg.ExtractUsage()
	if tokens != 15 {
		t.Fatalf("expected 15 tokens, got %d", tokens)
	}
	if cost != 0.02 {
		t.Fatalf("expected cost 0.02, got %f", cost)
	}
}

func TestParseStreamMessageToolUse(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"tool_use":{"name":"Bash","input":{"cmd":"ls"}}}}`)
	msg, err := ParseStreamMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tool, args := msg.ExtractToolEvent()
	if tool != "Bash" {
		t.Fatalf("expected tool Bash, got %s", tool)
	}
	if args == "" {
		t.Fatalf("expected tool args")
	}
}
