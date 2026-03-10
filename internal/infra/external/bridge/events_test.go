package bridge

import "testing"

func TestParseSDKEvent_Tool(t *testing.T) {
	line := []byte(`{"type":"tool","tool_name":"write_file","summary":"wrote main.go","iter":3}`)
	ev, err := ParseSDKEvent(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != SDKEventTool {
		t.Errorf("Type = %q, want %q", ev.Type, SDKEventTool)
	}
	if ev.ToolName != "write_file" {
		t.Errorf("ToolName = %q, want %q", ev.ToolName, "write_file")
	}
	if ev.Summary != "wrote main.go" {
		t.Errorf("Summary = %q, want %q", ev.Summary, "wrote main.go")
	}
	if ev.Iter != 3 {
		t.Errorf("Iter = %d, want 3", ev.Iter)
	}
}

func TestParseSDKEvent_Result(t *testing.T) {
	line := []byte(`{"type":"result","answer":"done","tokens":500,"cost":0.01,"iters":5}`)
	ev, err := ParseSDKEvent(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != SDKEventResult {
		t.Errorf("Type = %q, want %q", ev.Type, SDKEventResult)
	}
	if ev.Answer != "done" {
		t.Errorf("Answer = %q, want %q", ev.Answer, "done")
	}
	if ev.Tokens != 500 {
		t.Errorf("Tokens = %d, want 500", ev.Tokens)
	}
	if ev.Iters != 5 {
		t.Errorf("Iters = %d, want 5", ev.Iters)
	}
}

func TestParseSDKEvent_Error(t *testing.T) {
	line := []byte(`{"type":"error","is_error":true,"message":"timeout"}`)
	ev, err := ParseSDKEvent(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != SDKEventError {
		t.Errorf("Type = %q, want %q", ev.Type, SDKEventError)
	}
	if !ev.IsError {
		t.Error("IsError = false, want true")
	}
	if ev.Message != "timeout" {
		t.Errorf("Message = %q, want %q", ev.Message, "timeout")
	}
}

func TestParseSDKEvent_InvalidJSON(t *testing.T) {
	_, err := ParseSDKEvent([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseSDKEvent_EmptyObject(t *testing.T) {
	ev, err := ParseSDKEvent([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != "" {
		t.Errorf("Type = %q, want empty", ev.Type)
	}
}

func TestParseSDKEvent_Files(t *testing.T) {
	line := []byte(`{"type":"tool","files":["a.go","b.go"]}`)
	ev, err := ParseSDKEvent(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ev.Files) != 2 {
		t.Fatalf("Files len = %d, want 2", len(ev.Files))
	}
	if ev.Files[0] != "a.go" || ev.Files[1] != "b.go" {
		t.Errorf("Files = %v, want [a.go b.go]", ev.Files)
	}
}
