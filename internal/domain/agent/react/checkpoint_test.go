package react

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/ports/mocks"
	tools "alex/internal/agent/ports/tools"
)

type recordingCheckpointStore struct {
	mu      sync.Mutex
	saves   int
	deletes int
}

func (s *recordingCheckpointStore) Save(_ context.Context, _ *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves++
	return nil
}

func (s *recordingCheckpointStore) Load(_ context.Context, _ string) (*Checkpoint, error) {
	return nil, nil
}

func (s *recordingCheckpointStore) Delete(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	return nil
}

func (s *recordingCheckpointStore) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saves, s.deletes
}

func TestCheckpointWriteAfterObserve(t *testing.T) {
	store := &recordingCheckpointStore{}

	callCount := 0
	mockLLM := &mocks.MockLLMClient{
		StreamCompleteFunc: func(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
			callCount++
			if callCount == 1 {
				return &ports.CompletionResponse{
					ToolCalls: []ports.ToolCall{
						{ID: "call-1", Name: "mock_tool", Arguments: map[string]any{"foo": "bar"}},
					},
				}, nil
			}
			return &ports.CompletionResponse{Content: "done", StopReason: "stop"}, nil
		},
	}

	executor := &mocks.MockToolExecutor{
		ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
		},
	}
	registry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return executor, nil
		},
	}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations:   3,
		Logger:          agent.NoopLogger{},
		Clock:           agent.SystemClock{},
		CheckpointStore: store,
	})

	state := &TaskState{
		SessionID:    "session-checkpoint",
		RunID:        "run-checkpoint",
		SystemPrompt: "system",
	}

	if _, err := engine.SolveTask(context.Background(), "run tool", state, services); err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	saves, deletes := store.counts()
	if saves == 0 {
		t.Fatalf("expected checkpoint saves, got %d", saves)
	}
	if deletes != 1 {
		t.Fatalf("expected checkpoint to be cleared after completion, got %d deletes", deletes)
	}
}

func TestResumeFromCheckpoint(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)

	cp := &Checkpoint{
		ID:            "cp-1",
		SessionID:     "session-resume",
		Iteration:     2,
		MaxIterations: 4,
		Messages: []MessageState{
			{Role: "user", Content: "hello"},
		},
		CreatedAt: time.Now(),
		Version:   CheckpointVersion,
	}
	if err := store.Save(context.Background(), cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations:   10,
		Logger:          agent.NoopLogger{},
		Clock:           agent.SystemClock{},
		CheckpointStore: store,
	})

	state := &TaskState{SessionID: "session-resume"}
	services := Services{
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	resumed, err := engine.ResumeFromCheckpoint(context.Background(), "session-resume", state, services)
	if err != nil {
		t.Fatalf("ResumeFromCheckpoint returned error: %v", err)
	}
	if !resumed {
		t.Fatalf("expected resume to succeed")
	}
	if state.Iterations != 2 {
		t.Fatalf("expected iterations to be 2, got %d", state.Iterations)
	}
	if len(state.Messages) != 1 || state.Messages[0].Content != "hello" {
		t.Fatalf("expected checkpoint messages to be restored, got: %+v", state.Messages)
	}
	if engine.maxIterations != 4 {
		t.Fatalf("expected engine maxIterations to be 4, got %d", engine.maxIterations)
	}
}

func TestToolInFlightRecovery_Completed(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)

	result := "cached"
	cp := &Checkpoint{
		ID:        "cp-completed",
		SessionID: "session-completed",
		Iteration: 1,
		Messages: []MessageState{
			{Role: "user", Content: "hello"},
		},
		PendingTools: []ToolCallState{
			{
				ID:        "call-1",
				Name:      "mock_tool",
				Arguments: map[string]any{"foo": "bar"},
				Status:    "completed",
				Result:    &result,
			},
		},
		CreatedAt: time.Now(),
		Version:   CheckpointVersion,
	}
	if err := store.Save(context.Background(), cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	execCalls := 0
	registry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			execCalls++
			return &mocks.MockToolExecutor{}, nil
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		Logger:          agent.NoopLogger{},
		Clock:           agent.SystemClock{},
		CheckpointStore: store,
	})
	state := &TaskState{SessionID: "session-completed"}
	services := Services{
		ToolExecutor: registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	resumed, err := engine.ResumeFromCheckpoint(context.Background(), "session-completed", state, services)
	if err != nil {
		t.Fatalf("ResumeFromCheckpoint returned error: %v", err)
	}
	if !resumed {
		t.Fatalf("expected resume to succeed")
	}
	if execCalls != 0 {
		t.Fatalf("expected completed tool to skip execution, got %d executions", execCalls)
	}
	if len(state.ToolResults) != 1 || state.ToolResults[0].Content != "cached" {
		t.Fatalf("expected cached tool result to be applied, got: %+v", state.ToolResults)
	}
	if len(state.Messages) != 2 || state.Messages[1].Role != "tool" {
		t.Fatalf("expected tool message to be appended, got: %+v", state.Messages)
	}
}

func TestToolInFlightRecovery_Pending(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)

	cp := &Checkpoint{
		ID:        "cp-pending",
		SessionID: "session-pending",
		Iteration: 1,
		Messages: []MessageState{
			{Role: "user", Content: "hello"},
		},
		PendingTools: []ToolCallState{
			{
				ID:        "call-1",
				Name:      "mock_tool",
				Arguments: map[string]any{"foo": "bar"},
				Status:    "pending",
			},
		},
		CreatedAt: time.Now(),
		Version:   CheckpointVersion,
	}
	if err := store.Save(context.Background(), cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	execCalls := 0
	executor := &mocks.MockToolExecutor{
		ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			execCalls++
			return &ports.ToolResult{CallID: call.ID, Content: "fresh"}, nil
		},
	}
	registry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return executor, nil
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		Logger:          agent.NoopLogger{},
		Clock:           agent.SystemClock{},
		CheckpointStore: store,
	})
	state := &TaskState{SessionID: "session-pending"}
	services := Services{
		ToolExecutor: registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	resumed, err := engine.ResumeFromCheckpoint(context.Background(), "session-pending", state, services)
	if err != nil {
		t.Fatalf("ResumeFromCheckpoint returned error: %v", err)
	}
	if !resumed {
		t.Fatalf("expected resume to succeed")
	}
	if execCalls != 1 {
		t.Fatalf("expected pending tool to be executed once, got %d", execCalls)
	}
	if len(state.ToolResults) != 1 || state.ToolResults[0].Content != "fresh" {
		t.Fatalf("expected executed tool result to be applied, got: %+v", state.ToolResults)
	}
}

func TestCheckpointDeleteAfterResume(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)

	cp := &Checkpoint{
		ID:        "cp-delete",
		SessionID: "session-delete",
		Iteration: 0,
		Messages:  []MessageState{{Role: "user", Content: "hello"}},
		CreatedAt: time.Now(),
		Version:   CheckpointVersion,
	}
	if err := store.Save(context.Background(), cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	engine := NewReactEngine(ReactEngineConfig{
		Logger:          agent.NoopLogger{},
		Clock:           agent.SystemClock{},
		CheckpointStore: store,
	})
	state := &TaskState{SessionID: "session-delete"}
	services := Services{
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	resumed, err := engine.ResumeFromCheckpoint(context.Background(), "session-delete", state, services)
	if err != nil {
		t.Fatalf("ResumeFromCheckpoint returned error: %v", err)
	}
	if !resumed {
		t.Fatalf("expected resume to succeed")
	}

	path := filepath.Join(dir, "session-delete.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected checkpoint file to be deleted, got err=%v", err)
	}
}

func TestNoCheckpointStore(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{SessionID: "session-nil"}
	services := Services{
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	resumed, err := engine.ResumeFromCheckpoint(context.Background(), "session-nil", state, services)
	if err != nil {
		t.Fatalf("ResumeFromCheckpoint returned error: %v", err)
	}
	if resumed {
		t.Fatalf("expected resume to be false when store is nil")
	}
}

func TestCheckpointSaveIncludesPendingTools(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)

	engine := NewReactEngine(ReactEngineConfig{
		Logger:          agent.NoopLogger{},
		Clock:           agent.SystemClock{},
		CheckpointStore: store,
	})
	state := &TaskState{SessionID: "session-pending-save"}
	pending := []ToolCallState{{
		ID:        "call-1",
		Name:      "mock_tool",
		Arguments: map[string]any{"foo": "bar"},
		Status:    "pending",
	}}

	engine.saveCheckpoint(context.Background(), state, pending)

	cp, err := store.Load(context.Background(), "session-pending-save")
	if err != nil {
		t.Fatalf("Load checkpoint: %v", err)
	}
	if cp == nil {
		t.Fatalf("expected checkpoint to be saved")
	}
	if len(cp.PendingTools) != 1 {
		t.Fatalf("expected 1 pending tool, got %d", len(cp.PendingTools))
	}
	if cp.PendingTools[0].ID != "call-1" || cp.PendingTools[0].Status != "pending" {
		t.Fatalf("unexpected pending tool data: %+v", cp.PendingTools[0])
	}
}
