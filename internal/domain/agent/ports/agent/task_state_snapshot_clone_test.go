package agent

import (
	"testing"

	core "alex/internal/domain/agent/ports"
)

func TestCloneMessageClonesMapFields(t *testing.T) {
	original := core.Message{
		Metadata: map[string]any{
			"key": "value",
		},
	}

	cloned := cloneMessage(original)
	cloned.Metadata["key"] = "mutated"

	if original.Metadata["key"] != "value" {
		t.Fatalf("expected original metadata to remain unchanged, got %v", original.Metadata["key"])
	}
}

func TestCloneToolCallsClonesArgumentsMap(t *testing.T) {
	original := []core.ToolCall{
		{
			ID: "call-1",
			Arguments: map[string]any{
				"arg": "value",
			},
		},
	}

	cloned := cloneToolCalls(original)
	cloned[0].Arguments["arg"] = "mutated"

	if original[0].Arguments["arg"] != "value" {
		t.Fatalf("expected original arguments to remain unchanged, got %v", original[0].Arguments["arg"])
	}
}

func TestCloneToolResultsClonesMapFields(t *testing.T) {
	original := []core.ToolResult{
		{
			CallID: "call-1",
			Metadata: map[string]any{
				"status": "ok",
			},
		},
	}

	cloned := CloneToolResults(original)
	cloned[0].Metadata["status"] = "mutated"

	if original[0].Metadata["status"] != "ok" {
		t.Fatalf("expected original metadata to remain unchanged, got %v", original[0].Metadata["status"])
	}
}
