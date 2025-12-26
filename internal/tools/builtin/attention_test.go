package builtin

import (
	"testing"
	"time"

	"alex/internal/agent/ports"
)

func TestAttentionToolPinsImportantNotes(t *testing.T) {
	tool := NewAttention()
	call := ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"content": "User prefers dark mode dashboards.",
			"tags":    []any{"preference", "ui"},
		},
	}

	result, err := tool.Execute(nil, call)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	rawNotes, ok := result.Metadata["important_notes"].([]ports.ImportantNote)
	if !ok || len(rawNotes) != 1 {
		t.Fatalf("expected important_notes metadata, got %#v", result.Metadata)
	}
	note := rawNotes[0]
	if note.ID == "" {
		t.Fatalf("expected note ID to be set")
	}
	if note.Source != "attention" {
		t.Fatalf("expected source to be attention, got %q", note.Source)
	}
	if len(note.Tags) != 2 {
		t.Fatalf("expected tags to be preserved, got %#v", note.Tags)
	}
	if note.CreatedAt.IsZero() || note.CreatedAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("expected CreatedAt to be populated, got %v", note.CreatedAt)
	}
}
