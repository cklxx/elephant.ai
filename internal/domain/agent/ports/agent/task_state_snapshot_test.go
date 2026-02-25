package agent

import (
	"context"
	"testing"

	core "alex/internal/domain/agent/ports"
)

func TestTaskStateSnapshotRoundTrip(t *testing.T) {
	original := &TaskState{
		SystemPrompt: "You are main agent",
		Messages: []core.Message{{
			Role:    "user",
			Content: "Initial context",
			Metadata: map[string]any{
				"foo": "bar",
			},
			Attachments: map[string]core.Attachment{
				"diagram.png": {Name: "diagram.png", URI: "https://example.com/diagram.png"},
			},
		}},
		Attachments: map[string]core.Attachment{
			"notes.md": {Name: "notes.md", Data: "YmFzZTY0"},
		},
		AttachmentIterations: map[string]int{"notes.md": 2},
		Plans: []PlanNode{{ID: "plan-1", Title: "Investigate"}},
		Cognitive: &CognitiveExtension{
			Beliefs:         []Belief{{Statement: "delegation helps"}},
			KnowledgeRefs:   []KnowledgeReference{{ID: "rag-1", Description: "Docs"}},
			WorldState:      map[string]any{"last_tool": "search"},
			WorldDiff:       map[string]any{"iteration": 3},
			FeedbackSignals: []FeedbackSignal{{Kind: "success"}},
		},
	}

	ctx := WithTaskStateSnapshot(context.Background(), original)
	snapshot := GetTaskStateSnapshot(ctx)
	if snapshot == nil {
		t.Fatal("expected snapshot to be present")
	}

	snapshot.SystemPrompt = "mutated"
	snapshot.Messages[0].Content = "mutated"
	snapshot.Attachments["notes.md"] = core.Attachment{Name: "notes.md", Data: "new"}
	snapshot.AttachmentIterations["notes.md"] = 10

	if original.SystemPrompt == "mutated" {
		t.Fatalf("expected original SystemPrompt to remain unchanged")
	}
	if original.Messages[0].Content == "mutated" {
		t.Fatalf("expected original message to remain unchanged")
	}
	if original.Attachments["notes.md"].Data == "new" {
		t.Fatalf("expected original attachment to remain unchanged")
	}
	if original.AttachmentIterations["notes.md"] == 10 {
		t.Fatalf("expected original attachment iteration to remain unchanged")
	}
}

func TestWithClonedTaskStateSnapshotStoresExistingClone(t *testing.T) {
	cloned := &TaskState{SystemPrompt: "snapshot"}
	ctx := WithClonedTaskStateSnapshot(context.Background(), cloned)
	recovered := GetTaskStateSnapshot(ctx)
	if recovered == nil {
		t.Fatal("expected recovered snapshot")
	}
	recovered.SystemPrompt = "changed"
	if cloned.SystemPrompt != "snapshot" {
		t.Fatalf("expected provided clone to remain unchanged")
	}
}
