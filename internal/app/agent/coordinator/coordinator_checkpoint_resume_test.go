package coordinator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appconfig "alex/internal/agent/app/config"
	react "alex/internal/agent/domain/react"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/ports/mocks"
	tools "alex/internal/agent/ports/tools"
)

func TestCoordinatorResumeFromCheckpoint(t *testing.T) {
	t.Helper()

	sessionID := "session-resume"
	store := react.NewFileCheckpointStore(t.TempDir())
	checkpoint := &react.Checkpoint{
		ID:            "checkpoint-1",
		SessionID:     sessionID,
		Iteration:     1,
		MaxIterations: 1,
		Messages: []react.MessageState{
			{Role: "user", Content: "checkpoint user message"},
			{Role: "assistant", Content: "checkpoint assistant message"},
		},
		CreatedAt: time.Now(),
		Version:   react.CheckpointVersion,
	}
	if err := store.Save(context.Background(), checkpoint); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	llm := &mocks.MockLLMClient{CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
		t.Fatalf("LLM should not be called when resuming a completed checkpoint")
		return nil, errors.New("unexpected LLM call")
	}}

	registry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return nil, errors.New("unexpected tool call")
		},
		ListFunc: func() []ports.ToolDefinition { return nil },
	}

	coordinator := NewAgentCoordinator(
		stubLLMFactory{client: llm},
		registry,
		&stubSessionStore{},
		stubContextManager{},
		nil,
		&mocks.MockParser{},
		nil,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "checkpoint-resume",
			MaxIterations: 2,
		},
		WithCheckpointStore(store),
	)

	ctx := agent.WithOutputContext(context.Background(), &agent.OutputContext{Level: agent.LevelCore})
	result, err := coordinator.ExecuteTask(ctx, "resume checkpoint", sessionID, nil)
	if err != nil {
		t.Fatalf("ExecuteTask returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}

	if !messageContains(result.Messages, "checkpoint user message") {
		t.Fatalf("expected checkpoint message to be restored in result")
	}
	if messageContains(result.Messages, "resume checkpoint") {
		t.Fatalf("did not expect task input to be appended when resuming")
	}

	loaded, err := store.Load(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("load checkpoint after resume: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected checkpoint to be deleted after resume")
	}
}

func messageContains(messages []ports.Message, needle string) bool {
	for _, msg := range messages {
		if strings.Contains(msg.Content, needle) {
			return true
		}
	}
	return false
}
