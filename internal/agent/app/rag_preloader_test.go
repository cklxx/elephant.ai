package app

import (
	"testing"

	"alex/internal/agent/ports"
)

func TestRAGPreloaderAppendResultAddsSystemMessageWithoutToolCallID(t *testing.T) {
	preloader := newRAGPreloader(nil)
	env := &ports.ExecutionEnvironment{
		State: &ports.TaskState{},
	}

	result := ports.ToolResult{
		CallID:  "call-123",
		Content: "preloaded context",
		Attachments: map[string]ports.Attachment{
			"note.md": {
				Name: "note.md",
			},
		},
	}

	preloader.appendResult(env, result)

	if len(env.State.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(env.State.Messages))
	}

	msg := env.State.Messages[0]
	if msg.Role != "user" {
		t.Fatalf("expected user role, got %q", msg.Role)
	}
	if msg.ToolCallID != "" {
		t.Fatalf("expected tool_call_id to be empty, got %q", msg.ToolCallID)
	}
	if msg.Source != ports.MessageSourceToolResult {
		t.Fatalf("expected tool result source, got %q", msg.Source)
	}
	if msg.Attachments == nil || msg.Attachments["note.md"].Name != "note.md" {
		t.Fatalf("expected attachment to be preserved, got %+v", msg.Attachments)
	}

	if len(env.State.ToolResults) != 1 || env.State.ToolResults[0].CallID != "call-123" {
		t.Fatalf("expected tool result to be tracked, got %+v", env.State.ToolResults)
	}
}
