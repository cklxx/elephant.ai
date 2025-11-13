package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"testing"
)

func TestFileWriteRejectsEmptyPath(t *testing.T) {
	tool := &fileWrite{mode: tools.ExecutionModeLocal}

	call := ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"path":    "   ",
			"content": "hello",
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	if result.Error == nil {
		t.Fatalf("expected error, got success: %#v", result)
	}

	if result.Error.Error() != "file path cannot be empty" {
		t.Fatalf("unexpected error message: %v", result.Error)
	}
}

func TestResolveSandboxPathRelative(t *testing.T) {
	got, err := resolveSandboxPath("notes.txt")
	if err != nil {
		t.Fatalf("resolveSandboxPath returned error: %v", err)
	}

	want := "/workspace/notes.txt"
	if got != want {
		t.Fatalf("resolveSandboxPath mismatch: got %q want %q", got, want)
	}
}

func TestResolveSandboxPathAbsolute(t *testing.T) {
	got, err := resolveSandboxPath("/workspace/project/output.txt")
	if err != nil {
		t.Fatalf("resolveSandboxPath returned error: %v", err)
	}

	want := "/workspace/project/output.txt"
	if got != want {
		t.Fatalf("resolveSandboxPath mismatch: got %q want %q", got, want)
	}
}

func TestResolveSandboxPathRejectsEscape(t *testing.T) {
	if _, err := resolveSandboxPath("../etc/passwd"); err == nil {
		t.Fatalf("expected error when escaping workspace")
	}

	if _, err := resolveSandboxPath("/etc/passwd"); err == nil {
		t.Fatalf("expected error for absolute path outside workspace")
	}
}
