package app

import (
	"context"
	"testing"

	sessionstate "alex/internal/infra/session/state_store"
)

// mockBridgeResumer implements BridgeOrphanResumer for testing.
type mockBridgeResumer struct {
	called  bool
	workDir string
	results []OrphanResumeResult
}

func (m *mockBridgeResumer) ResumeOrphans(_ context.Context, workDir string) []OrphanResumeResult {
	m.called = true
	m.workDir = workDir
	return m.results
}

func TestResumePendingTasks_CallsBridgeResumer(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	resumer := &mockBridgeResumer{
		results: []OrphanResumeResult{
			{TaskID: "orphan-1", Action: "harvest"},
			{TaskID: "orphan-2", Action: "adopt"},
		},
	}

	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
		WithBridgeResumer(resumer, "/workspace"),
	)

	_, err := svc.ResumePendingTasks(ctx)
	if err != nil {
		t.Fatalf("resume pending tasks: %v", err)
	}

	if !resumer.called {
		t.Fatal("expected bridge resumer to be called")
	}
	if resumer.workDir != "/workspace" {
		t.Errorf("workDir = %q, want /workspace", resumer.workDir)
	}
}

func TestResumePendingTasks_SkipsResumerWhenNil(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
		// No bridge resumer wired.
	)

	// Should not panic.
	resumed, err := svc.ResumePendingTasks(ctx)
	if err != nil {
		t.Fatalf("resume pending tasks: %v", err)
	}
	if resumed != 0 {
		t.Errorf("expected 0 resumed, got %d", resumed)
	}
}

func TestResumePendingTasks_SkipsResumerWithEmptyWorkDir(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	resumer := &mockBridgeResumer{}

	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
		WithBridgeResumer(resumer, ""), // empty workDir
	)

	_, err := svc.ResumePendingTasks(ctx)
	if err != nil {
		t.Fatalf("resume pending tasks: %v", err)
	}

	if resumer.called {
		t.Fatal("expected bridge resumer NOT to be called with empty workDir")
	}
}

func TestResumeOrphanedBridges_CountsActions(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	resumer := &mockBridgeResumer{
		results: []OrphanResumeResult{
			{TaskID: "t1", Action: "adopt"},
			{TaskID: "t2", Action: "harvest"},
			{TaskID: "t3", Action: "harvest"},
			{TaskID: "t4", Action: "mark_failed"},
			{TaskID: "t5", Action: "retry_with_context"},
		},
	}

	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
		WithBridgeResumer(resumer, "/workspace"),
	)

	// This calls resumeOrphanedBridges internally.
	_, err := svc.ResumePendingTasks(ctx)
	if err != nil {
		t.Fatalf("resume pending tasks: %v", err)
	}

	if !resumer.called {
		t.Fatal("expected bridge resumer to be called")
	}
}
