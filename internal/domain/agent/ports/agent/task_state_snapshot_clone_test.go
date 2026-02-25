package agent

import (
	"testing"

	core "alex/internal/domain/agent/ports"
)

func TestCloneMessageClonesMapFields(t *testing.T) {
	original := core.Message{
		Metadata: map[string]any{
			"source": "user",
		},
		Attachments: map[string]core.Attachment{
			"note.txt": {Name: "note.txt", Data: "original"},
		},
		ToolCalls: []core.ToolCall{{
			ID:        "call-1",
			Name:      "search",
			Arguments: map[string]any{"query": "hello"},
		}},
		ToolResults: []core.ToolResult{{
			CallID:   "call-1",
			Metadata: map[string]any{"status": "ok"},
		}},
	}

	cloned := cloneMessage(original)
	cloned.Metadata["source"] = "assistant"
	cloned.Attachments["note.txt"] = core.Attachment{Name: "note.txt", Data: "cloned"}
	cloned.ToolCalls[0].Arguments["query"] = "mutated"
	cloned.ToolResults[0].Metadata["status"] = "error"

	if got := original.Metadata["source"]; got != "user" {
		t.Fatalf("expected original metadata to stay unchanged, got %v", got)
	}
	if got := original.Attachments["note.txt"].Data; got != "original" {
		t.Fatalf("expected original attachment data to stay unchanged, got %q", got)
	}
	if got := original.ToolCalls[0].Arguments["query"]; got != "hello" {
		t.Fatalf("expected original tool call arguments to stay unchanged, got %v", got)
	}
	if got := original.ToolResults[0].Metadata["status"]; got != "ok" {
		t.Fatalf("expected original tool result metadata to stay unchanged, got %v", got)
	}
}

func TestCloneToolCallsClonesArgumentsMap(t *testing.T) {
	original := []core.ToolCall{{
		ID:        "call-1",
		Name:      "search",
		Arguments: map[string]any{"query": "hello"},
	}}

	cloned := cloneToolCalls(original)
	cloned[0].Arguments["query"] = "mutated"

	if got := original[0].Arguments["query"]; got != "hello" {
		t.Fatalf("expected original tool call arguments to stay unchanged, got %v", got)
	}
}

func TestCloneToolResultsClonesMapFields(t *testing.T) {
	original := []core.ToolResult{{
		CallID:   "call-1",
		Content:  "done",
		Metadata: map[string]any{"status": "ok"},
		Attachments: map[string]core.Attachment{
			"result.txt": {Name: "result.txt", Data: "original"},
		},
	}}

	cloned := CloneToolResults(original)
	cloned[0].Metadata["status"] = "error"
	cloned[0].Attachments["result.txt"] = core.Attachment{Name: "result.txt", Data: "cloned"}

	if got := original[0].Metadata["status"]; got != "ok" {
		t.Fatalf("expected original tool result metadata to stay unchanged, got %v", got)
	}
	if got := original[0].Attachments["result.txt"].Data; got != "original" {
		t.Fatalf("expected original tool result attachments to stay unchanged, got %q", got)
	}
}
