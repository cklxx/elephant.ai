package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	serverPorts "alex/internal/delivery/server/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	sessionstate "alex/internal/infra/session/state_store"
	id "alex/internal/shared/utils/id"
)

type resumeAgentExecutor struct {
	sessionStore storage.SessionStore

	mu             sync.Mutex
	executedRunIDs []string
	failSessions   map[string]error
}

func (m *resumeAgentExecutor) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if err := m.failSessions[id]; err != nil {
		return nil, err
	}
	return m.sessionStore.Get(ctx, id)
}

func (m *resumeAgentExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	m.mu.Lock()
	m.executedRunIDs = append(m.executedRunIDs, id.RunIDFromContext(ctx))
	m.mu.Unlock()

	return &agent.TaskResult{
		Answer:     "resumed",
		Iterations: 1,
		TokensUsed: 10,
		StopReason: "completed",
		SessionID:  sessionID,
	}, nil
}

func (m *resumeAgentExecutor) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (m *resumeAgentExecutor) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{}, nil
}

func (m *resumeAgentExecutor) executionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.executedRunIDs)
}

func (m *resumeAgentExecutor) lastRunID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.executedRunIDs) == 0 {
		return ""
	}
	return m.executedRunIDs[len(m.executedRunIDs)-1]
}

func TestTaskExecutionService_ResumePendingTasks_HappyPath(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	runID := "run-resume-happy"
	createCtx := id.WithRunID(ctx, runID)
	task, err := taskStore.Create(createCtx, "session-resume-1", "resume me", "planner", "safe")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := taskStore.SetStatus(ctx, task.ID, serverPorts.TaskStatusRunning); err != nil {
		t.Fatalf("set running status: %v", err)
	}

	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
	)

	resumed, err := svc.ResumePendingTasks(ctx)
	if err != nil {
		t.Fatalf("resume pending tasks: %v", err)
	}
	if resumed != 1 {
		t.Fatalf("expected 1 resumed task, got %d", resumed)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := taskStore.Get(ctx, task.ID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if got.Status == serverPorts.TaskStatusCompleted {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	got, err := taskStore.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task after resume: %v", err)
	}
	if got.Status != serverPorts.TaskStatusCompleted {
		t.Fatalf("expected task to complete after resume, got status %s", got.Status)
	}
	if got.Result == nil {
		t.Fatal("expected task result to be recorded")
	}

	if agentExecutor.executionCount() != 1 {
		t.Fatalf("expected one execution, got %d", agentExecutor.executionCount())
	}
	if agentExecutor.lastRunID() != task.ID {
		t.Fatalf("expected resumed run_id %s, got %s", task.ID, agentExecutor.lastRunID())
	}
}

func TestTaskExecutionService_ResumePendingTasks_SkipsInvalidOrUnresolvableTasks(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	blankDescCtx := id.WithRunID(ctx, "run-resume-empty-desc")
	blankDescTask, err := taskStore.Create(blankDescCtx, "session-resume-2", "", "", "")
	if err != nil {
		t.Fatalf("create blank desc task: %v", err)
	}
	if err := taskStore.SetStatus(ctx, blankDescTask.ID, serverPorts.TaskStatusPending); err != nil {
		t.Fatalf("set pending status for blank desc: %v", err)
	}

	failingCtx := id.WithRunID(ctx, "run-resume-failing-session")
	failingTask, err := taskStore.Create(failingCtx, "broken-session", "will fail session lookup", "", "")
	if err != nil {
		t.Fatalf("create failing task: %v", err)
	}
	if err := taskStore.SetStatus(ctx, failingTask.ID, serverPorts.TaskStatusRunning); err != nil {
		t.Fatalf("set running status for failing task: %v", err)
	}

	agentExecutor := &resumeAgentExecutor{
		sessionStore: sessionStore,
		failSessions: map[string]error{
			"broken-session": errors.New("session lookup failed"),
		},
	}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
	)

	resumed, err := svc.ResumePendingTasks(ctx)
	if err != nil {
		t.Fatalf("resume pending tasks: %v", err)
	}
	if resumed != 0 {
		t.Fatalf("expected no tasks resumed, got %d", resumed)
	}
	if agentExecutor.executionCount() != 0 {
		t.Fatalf("expected no executions, got %d", agentExecutor.executionCount())
	}

	gotBlank, err := taskStore.Get(ctx, blankDescTask.ID)
	if err != nil {
		t.Fatalf("get blank desc task: %v", err)
	}
	if gotBlank.Status != serverPorts.TaskStatusPending {
		t.Fatalf("expected blank desc task to remain pending, got %s", gotBlank.Status)
	}

	gotFailing, err := taskStore.Get(ctx, failingTask.ID)
	if err != nil {
		t.Fatalf("get failing task: %v", err)
	}
	if gotFailing.Status != serverPorts.TaskStatusRunning {
		t.Fatalf("expected failing task to remain running, got %s", gotFailing.Status)
	}
}
