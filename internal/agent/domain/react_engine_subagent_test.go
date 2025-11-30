package domain

import (
	"testing"

	"alex/internal/agent/ports"
)

func TestBuildSubagentStateSnapshotRemovesCurrentCallAndAppendsPrompt(t *testing.T) {
	original := &TaskState{
		Messages: []Message{
			{Role: "user", Content: "hello", Source: ports.MessageSourceUserInput},
			{
				Role:      "assistant",
				Content:   "delegating",
				ToolCalls: []ports.ToolCall{{ID: "call-1", Name: "subagent"}},
			},
		},
	}

	call := ToolCall{ID: "call-1", Arguments: map[string]any{"prompt": "investigate"}}

	snapshot := buildSubagentStateSnapshot(original, call)

	if snapshot == nil {
		t.Fatalf("expected snapshot to be created")
	}

	if got := len(snapshot.Messages); got != 2 {
		t.Fatalf("expected message count to remain stable after pruning and appending prompt, got %d", got)
	}

	if containsSubagentToolCall(snapshot.Messages[0].ToolCalls, "call-1") {
		t.Fatalf("expected subagent tool call message to be removed")
	}

	prompt := snapshot.Messages[1]
	if prompt.Role != "user" || prompt.Content != "investigate" || prompt.Source != ports.MessageSourceUserInput {
		t.Fatalf("unexpected prompt message: %#v", prompt)
	}

	if len(original.Messages) != 2 {
		t.Fatalf("expected original messages to remain unchanged")
	}
}

func TestBuildSubagentStateSnapshotWithoutPrompt(t *testing.T) {
	call := ToolCall{ID: "call-1", Arguments: map[string]any{}}

	state := buildSubagentStateSnapshot(&TaskState{Messages: []Message{{
		Role:      "assistant",
		ToolCalls: []ports.ToolCall{{ID: "call-1", Name: "subagent"}},
	}}}, call)

	if state == nil {
		t.Fatalf("expected snapshot to be created")
	}

	if got := len(state.Messages); got != 0 {
		t.Fatalf("expected tool call to be pruned without appending prompt, got %d messages", got)
	}
}
