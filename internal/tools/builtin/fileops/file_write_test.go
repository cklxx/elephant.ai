package fileops

import (
	"alex/internal/agent/ports"
	"context"
	"testing"
)

func TestFileWriteRejectsEmptyPath(t *testing.T) {
	tool := &fileWrite{}

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
