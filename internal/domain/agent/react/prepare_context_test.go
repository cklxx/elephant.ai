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

func TestOffloadMessageThinkingPreservesSignedParts(t *testing.T) {
	state := &TaskState{
		Messages: []Message{{
			Role: "assistant",
			Thinking: ports.Thinking{Parts: []ports.ThinkingPart{
				{Kind: "thinking", Text: "unsigned reasoning"},
				{Kind: "thinking", Text: "signed reasoning", Signature: "sig-abc"},
			}},
		}},
	}

	offloadMessageThinking(state)

	parts := state.Messages[0].Thinking.Parts
	if len(parts) != 1 {
		t.Fatalf("expected 1 signed part preserved, got %d", len(parts))
	}
	if parts[0].Signature != "sig-abc" {
		t.Fatalf("expected signed part preserved, got signature=%q", parts[0].Signature)
	}
}
