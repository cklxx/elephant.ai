package builtin

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/memory"
	id "alex/internal/utils/id"
)

func TestMemoryWriteRequiresPersonalizationMetadata(t *testing.T) {
	store := memory.NewService(memory.NewInMemoryStore())
	tool := NewMemoryWrite(store)
	ctx := id.WithUserID(context.Background(), "user-1")
	call := ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"content": "General fact without personalization",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected validation error when keywords and slots are missing")
	}
	if !strings.Contains(result.Content, "keywords or slots") {
		t.Fatalf("expected personalization guidance in error, got %q", result.Content)
	}
}
