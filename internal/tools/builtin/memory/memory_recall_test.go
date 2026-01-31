package memory

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/memory"
	id "alex/internal/utils/id"
)

func TestMemoryRecallRequiresUserID(t *testing.T) {
	service := memory.NewService(memory.NewInMemoryStore())
	tool := NewMemoryRecall(service)

	call := ports.ToolCall{
		ID: "call-missing-user",
		Arguments: map[string]any{
			"keywords": []any{"alpha"},
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected error for missing user id")
	}
}

func TestMemoryRecallReturnsEntries(t *testing.T) {
	store := memory.NewInMemoryStore()
	service := memory.NewService(store)
	ctx := id.WithUserID(context.Background(), "user-1")

	_, err := service.Save(ctx, memory.Entry{
		UserID:   "user-1",
		Content:  "alpha project kickoff notes",
		Keywords: []string{"alpha"},
		Slots:    map[string]string{"project": "alpha"},
	})
	if err != nil {
		t.Fatalf("save memory: %v", err)
	}

	tool := NewMemoryRecall(service)
	call := ports.ToolCall{
		ID: "call-recall",
		Arguments: map[string]any{
			"keywords": []any{"alpha"},
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Metadata == nil {
		t.Fatalf("expected metadata to be populated")
	}
	memories, ok := result.Metadata["memories"].([]map[string]any)
	if !ok || len(memories) == 0 {
		t.Fatalf("expected memories in metadata, got %#v", result.Metadata["memories"])
	}
}

func TestMemoryRecallSupportsQueryText(t *testing.T) {
	store := memory.NewInMemoryStore()
	service := memory.NewService(store)
	ctx := id.WithUserID(context.Background(), "user-1")

	_, err := service.Save(ctx, memory.Entry{
		UserID:   "user-1",
		Content:  "alpha project kickoff notes",
		Keywords: []string{"alpha"},
	})
	if err != nil {
		t.Fatalf("save memory: %v", err)
	}

	tool := NewMemoryRecall(service)
	call := ports.ToolCall{
		ID: "call-query",
		Arguments: map[string]any{
			"query": "alpha project",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
}
