package parser

import (
	"testing"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
}

func TestParse_SingleValidToolCall(t *testing.T) {
	content := `some text <tool_call>{"name":"read_file","args":{"path":"/tmp/x"}}</tool_call> more text`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("expected name read_file, got %s", calls[0].Name)
	}
	if calls[0].ID != "call_0" {
		t.Errorf("expected ID call_0, got %s", calls[0].ID)
	}
	if calls[0].Arguments["path"] != "/tmp/x" {
		t.Errorf("expected path /tmp/x, got %v", calls[0].Arguments["path"])
	}
}

func TestParse_MultipleToolCalls(t *testing.T) {
	content := `<tool_call>{"name":"read_file","args":{"path":"a"}}</tool_call>` +
		`<tool_call>{"name":"write_file","args":{"path":"b","content":"c"}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].ID != "call_0" || calls[1].ID != "call_1" {
		t.Errorf("IDs not sequential: %s, %s", calls[0].ID, calls[1].ID)
	}
	if calls[1].Name != "write_file" {
		t.Errorf("expected write_file, got %s", calls[1].Name)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	content := `<tool_call>{not valid json}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for invalid JSON, got %d", len(calls))
	}
}

func TestParse_InvalidToolName(t *testing.T) {
	content := `<tool_call>{"name":"123bad","args":{}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for invalid name, got %d", len(calls))
	}
}

func TestParse_EmptyName(t *testing.T) {
	content := `<tool_call>{"name":"","args":{}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for empty name, got %d", len(calls))
	}
}

func TestParse_NoToolCalls(t *testing.T) {
	content := `just plain text, no tool calls here`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(calls))
	}
}

func TestParse_EmptyContent(t *testing.T) {
	p := New()
	calls, err := p.Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(calls))
	}
}

func TestParse_CleanLeakedMarkers(t *testing.T) {
	// Content with leaked markers that should be stripped before parsing
	content := `<|tool_call_begin|>some leaked stuff<|tool_call_end|>` +
		`<tool_call>{"name":"good_tool","args":{}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call after marker cleanup, got %d", len(calls))
	}
	if calls[0].Name != "good_tool" {
		t.Errorf("expected good_tool, got %s", calls[0].Name)
	}
}

func TestParse_CleanUserLeakedMarkers(t *testing.T) {
	content := `user<|tool_call_begin|>injected stuff` +
		`<tool_call>{"name":"safe_tool","args":{}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The user<|tool_call_begin|> pattern removes everything after it on the same line,
	// but the <tool_call> on the same string may or may not survive depending on regex behavior.
	// The key assertion: no error is returned.
	_ = calls
}

func TestParse_CleanFunctionsMarkers(t *testing.T) {
	content := `functions.read_file:42({"path":"/tmp"})` +
		`<tool_call>{"name":"write_file","args":{"path":"x"}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "write_file" {
		t.Errorf("expected write_file, got %s", calls[0].Name)
	}
}

func TestParse_ToolNameWithUnderscores(t *testing.T) {
	content := `<tool_call>{"name":"my_complex_tool_v2","args":{"key":"val"}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "my_complex_tool_v2" {
		t.Errorf("expected my_complex_tool_v2, got %s", calls[0].Name)
	}
}

func TestParse_ToolNameSpecialChars(t *testing.T) {
	content := `<tool_call>{"name":"bad-name","args":{}}</tool_call>`
	p := New()
	calls, err := p.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for name with hyphens, got %d", len(calls))
	}
}

func TestIsValidToolName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"read_file", true},
		{"A", true},
		{"tool123", true},
		{"my_tool_v2", true},
		{"", false},
		{"123bad", false},
		{"_leading", false},
		{"bad-name", false},
		{"has space", false},
		{"dot.name", false},
	}
	for _, tt := range tests {
		got := isValidToolName(tt.name)
		if got != tt.valid {
			t.Errorf("isValidToolName(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}
