package react

import (
	"testing"

	"alex/internal/domain/agent/ports"
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

func TestBuildSubagentStateSnapshotRemovesToolOutputForCall(t *testing.T) {
	original := &TaskState{
		Messages: []Message{
			{Role: "user", Content: "keep", Source: ports.MessageSourceUserInput},
			{
				Role:      "assistant",
				Content:   "delegating",
				ToolCalls: []ports.ToolCall{{ID: "call-1", Name: "subagent"}},
			},
			{
				Role:       "tool",
				Content:    "tool failed",
				ToolCallID: "call-1",
				ToolResults: []ports.ToolResult{{
					CallID:  "call-1",
					Content: "tool failed",
				}},
			},
			{
				Role:       "tool",
				Content:    "other tool ok",
				ToolCallID: "call-2",
				ToolResults: []ports.ToolResult{{
					CallID:  "call-2",
					Content: "other tool ok",
				}},
			},
		},
	}

	call := ToolCall{ID: "call-1", Arguments: map[string]any{}}

	snapshot := buildSubagentStateSnapshot(original, call)

	if snapshot == nil {
		t.Fatalf("expected snapshot to be created")
	}

	if got := len(snapshot.Messages); got != 2 {
		t.Fatalf("expected message count to drop subagent tool call + output, got %d", got)
	}

	if snapshot.Messages[0].Content != "keep" {
		t.Fatalf("expected user message to remain, got %#v", snapshot.Messages[0])
	}

	if snapshot.Messages[1].ToolCallID != "call-2" {
		t.Fatalf("expected other tool output to remain, got %#v", snapshot.Messages[1])
	}
}
