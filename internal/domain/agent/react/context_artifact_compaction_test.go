package react

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestTryArtifactCompactionWritesFileAndPlaceholder(t *testing.T) {
	root := t.TempDir()
	engine := NewReactEngine(ReactEngineConfig{
		CheckpointStore: newTestFileCheckpointStore(filepath.Join(root, "checkpoints")),
	})
	state := &TaskState{
		SessionID:  "sess-artifact",
		RunID:      "run-1",
		Iterations: 3,
		Messages: []Message{
			{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "system"},
			{Role: "user", Source: ports.MessageSourceUserInput, Content: "old user question"},
			{Role: "assistant", Source: ports.MessageSourceAssistantReply, Content: "old assistant reply"},
			{Role: "user", Source: ports.MessageSourceUserInput, Content: "latest user question"},
			{Role: "assistant", Source: ports.MessageSourceAssistantReply, Content: "latest assistant reply"},
		},
	}
	services := Services{
		Context: &mockContextManager{
			estimateFunc: func(msgs []ports.Message) int { return len(msgs) * 100 },
		},
	}

	compacted, ok := engine.tryArtifactCompaction(
		context.Background(),
		state,
		services,
		state.Messages,
		compactionReasonThreshold,
		false,
	)
	if !ok {
		t.Fatal("expected artifact compaction to be applied")
	}
	if state.ContextCompactionSeq != 1 {
		t.Fatalf("expected compaction sequence 1, got %d", state.ContextCompactionSeq)
	}
	if state.NextCompactionAllowed != 5 {
		t.Fatalf("expected next allowed iteration 5, got %d", state.NextCompactionAllowed)
	}
	if strings.TrimSpace(state.LastCompactionArtifact) == "" {
		t.Fatal("expected artifact path to be recorded on state")
	}

	data, err := os.ReadFile(state.LastCompactionArtifact)
	if err != nil {
		t.Fatalf("expected artifact file to exist: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "context_compaction_artifact") {
		t.Fatalf("expected artifact content marker, got: %s", content)
	}
	if !strings.Contains(content, "\"session_id\": \"sess-artifact\"") {
		t.Fatalf("expected session id in artifact, got: %s", content)
	}

	if len(compacted) != 4 {
		t.Fatalf("expected compacted message count 4, got %d", len(compacted))
	}
	if !strings.HasPrefix(strings.TrimSpace(compacted[1].Content), contextPlaceholderPrefix) {
		t.Fatalf("expected placeholder inserted at index 1, got %q", compacted[1].Content)
	}
	if compacted[1].Source != ports.MessageSourceCheckpoint {
		t.Fatalf("expected placeholder source checkpoint, got %q", compacted[1].Source)
	}
}

func TestTryArtifactCompactionRespectsCooldownUnlessForced(t *testing.T) {
	root := t.TempDir()
	engine := NewReactEngine(ReactEngineConfig{
		CheckpointStore: newTestFileCheckpointStore(filepath.Join(root, "checkpoints")),
	})
	state := &TaskState{
		SessionID:             "sess-cooldown",
		RunID:                 "run-2",
		Iterations:            3,
		NextCompactionAllowed: 5,
		ContextCompactionSeq:  1,
	}
	services := Services{
		Context: &mockContextManager{
			estimateFunc: func(msgs []ports.Message) int { return len(msgs) * 100 },
		},
	}
	messages := []Message{
		{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "system"},
		{Role: "user", Source: ports.MessageSourceUserInput, Content: "turn 1"},
		{Role: "assistant", Source: ports.MessageSourceAssistantReply, Content: "reply 1"},
		{Role: "user", Source: ports.MessageSourceUserInput, Content: "turn 2"},
		{Role: "assistant", Source: ports.MessageSourceAssistantReply, Content: "reply 2"},
	}

	if _, ok := engine.tryArtifactCompaction(context.Background(), state, services, messages, compactionReasonThreshold, false); ok {
		t.Fatal("expected cooldown to block threshold compaction")
	}
	if state.ContextCompactionSeq != 1 {
		t.Fatalf("expected sequence unchanged during cooldown, got %d", state.ContextCompactionSeq)
	}

	compacted, ok := engine.tryArtifactCompaction(context.Background(), state, services, messages, compactionReasonOverflow, true)
	if !ok {
		t.Fatal("expected forced overflow compaction to bypass cooldown")
	}
	if state.ContextCompactionSeq != 2 {
		t.Fatalf("expected sequence incremented to 2 after forced compaction, got %d", state.ContextCompactionSeq)
	}
	if len(compacted) != 4 {
		t.Fatalf("expected compacted message count 4, got %d", len(compacted))
	}
}
