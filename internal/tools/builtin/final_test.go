package builtin

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
)

func TestFinalToolRequiresAnswer(t *testing.T) {
	tool := NewFinal()
	_, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-final", Arguments: map[string]any{}})
	if err == nil {
		t.Fatalf("expected error when answer is empty")
	}
}

func TestFinalToolReturnsContentAndMetadata(t *testing.T) {
	tool := NewFinal()
	call := ports.ToolCall{
		ID: "call-final",
		Arguments: map[string]any{
			"answer":     "All tasks completed successfully.",
			"highlights": []string{"3 subagents", "badge design finalized"},
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error executing final tool: %v", err)
	}
	if result.CallID != call.ID {
		t.Fatalf("expected call id %q, got %q", call.ID, result.CallID)
	}
	if result.Content != "All tasks completed successfully." {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if result.Metadata == nil || result.Metadata["final_tool"] != true {
		t.Fatalf("expected final_tool metadata flag, got %+v", result.Metadata)
	}
	if _, ok := result.Metadata["highlights"]; !ok {
		t.Fatalf("expected highlights to propagate into metadata")
	}
}
