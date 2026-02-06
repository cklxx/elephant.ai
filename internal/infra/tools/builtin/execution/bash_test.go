//go:build local_exec

package execution

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"
)

func TestBashExecuteSuccess(t *testing.T) {
	tool := NewBash(shared.ShellToolConfig{})
	call := ports.ToolCall{ID: "call-1", Arguments: map[string]any{"command": "printf 'hello'"}}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got: %v", result.Error)
	}

	if result.Content != "hello" {
		t.Fatalf("expected content 'hello', got %q", result.Content)
	}

	command, ok := result.Metadata["command"].(string)
	if !ok {
		t.Fatalf("expected command metadata, got %T", result.Metadata["command"])
	}
	if !strings.HasSuffix(command, "printf 'hello'") {
		t.Fatalf("expected command to include original payload, got %q", command)
	}

	if stdout, ok := result.Metadata["stdout"].(string); !ok || stdout != "hello" {
		t.Fatalf("expected stdout metadata 'hello', got %v", result.Metadata["stdout"])
	}
	if stderr, ok := result.Metadata["stderr"].(string); !ok || stderr != "" {
		t.Fatalf("expected empty stderr metadata, got %v", result.Metadata["stderr"])
	}
	if text, ok := result.Metadata["text"].(string); !ok || text != "hello" {
		t.Fatalf("expected text metadata 'hello', got %v", result.Metadata["text"])
	}

	if result.Metadata["exit_code"] != 0 {
		t.Fatalf("expected metadata exit_code 0, got %v", result.Metadata["exit_code"])
	}
}

func TestBashExecuteFailure(t *testing.T) {
	tool := NewBash(shared.ShellToolConfig{})
	call := ports.ToolCall{ID: "call-2", Arguments: map[string]any{"command": "echo error 1>&2; exit 3"}}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for non-zero exit code")
	}

	if result.Content != "error" {
		t.Fatalf("expected content 'error', got %q", result.Content)
	}
	if result.Metadata["exit_code"] != 3 {
		t.Fatalf("expected exit code 3, got %v", result.Metadata["exit_code"])
	}
	stderr, ok := result.Metadata["stderr"].(string)
	if !ok || strings.TrimSpace(stderr) == "" {
		t.Fatal("expected stderr to be captured")
	}
	if text, ok := result.Metadata["text"].(string); !ok || strings.TrimSpace(text) == "" {
		t.Fatal("expected text metadata to include stderr output")
	}
}
