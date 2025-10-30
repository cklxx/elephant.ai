package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

type bashResult struct {
	Command  string `json:"command"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func TestBashExecuteSuccess(t *testing.T) {
	tool := NewBash(ShellToolConfig{})
	call := ports.ToolCall{ID: "call-1", Arguments: map[string]any{"command": "printf 'hello'"}}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got: %v", result.Error)
	}

	var payload bashResult
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	if !strings.HasSuffix(payload.Command, "printf 'hello'") {
		t.Fatalf("expected command to include original payload, got %q", payload.Command)
	}
	if !strings.HasPrefix(payload.Command, "cd ") {
		t.Fatalf("expected command to include working directory prefix, got %q", payload.Command)
	}
	if payload.Stdout != "hello" {
		t.Fatalf("expected stdout 'hello', got %q", payload.Stdout)
	}
	if payload.Stderr != "" {
		t.Fatalf("expected empty stderr, got %q", payload.Stderr)
	}
	if payload.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", payload.ExitCode)
	}

	if result.Metadata["exit_code"] != 0 {
		t.Fatalf("expected metadata exit_code 0, got %v", result.Metadata["exit_code"])
	}
}

func TestBashExecuteFailure(t *testing.T) {
	tool := NewBash(ShellToolConfig{})
	call := ports.ToolCall{ID: "call-2", Arguments: map[string]any{"command": "echo error 1>&2; exit 3"}}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for non-zero exit code")
	}

	var payload bashResult
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	if payload.ExitCode != 3 {
		t.Fatalf("expected exit code 3, got %d", payload.ExitCode)
	}
	if payload.Stderr == "" {
		t.Fatal("expected stderr to be captured")
	}
}
