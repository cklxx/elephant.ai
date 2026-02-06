package react

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestPrepareUserTaskContextOffloadsThinking(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Messages: []Message{{
			Role:     "assistant",
			Thinking: ports.Thinking{Parts: []ports.ThinkingPart{{Kind: "thinking", Text: "secret"}}},
		}},
	}

	engine.prepareUserTaskContext(context.Background(), "next", state)

	if len(state.Messages) < 1 {
		t.Fatalf("expected messages to remain present")
	}
	if len(state.Messages[0].Thinking.Parts) != 0 {
		t.Fatalf("expected thinking to be offloaded before new user input")
	}
	if last := state.Messages[len(state.Messages)-1]; last.Role != "user" {
		t.Fatalf("expected new user message appended, got role=%q", last.Role)
	}
}
