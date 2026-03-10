package app

import (
	"context"
	"errors"
	"fmt"
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

	otherOwnerLease := time.Now().Add(45 * time.Second)
	claimedBlank, err := taskStore.TryClaimTask(ctx, blankDescTask.ID, "other-owner", otherOwnerLease)
	if err != nil {
		t.Fatalf("claim blank desc task after skip: %v", err)
	}
	if !claimedBlank {
		t.Fatal("expected blank desc task lease to be released after skip")
	}

	claimedFailing, err := taskStore.TryClaimTask(ctx, failingTask.ID, "other-owner", otherOwnerLease)
	if err != nil {
		t.Fatalf("claim failing task after skip: %v", err)
	}
	if !claimedFailing {
		t.Fatal("expected failing task lease to be released after skip")
	}
}

func TestTaskExecutionService_ResumePendingTasks_RespectsClaimBatchSize(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	for i := 0; i < 3; i++ {
		runID := fmt.Sprintf("run-resume-batch-%d", i)
		createCtx := id.WithRunID(ctx, runID)
		task, err := taskStore.Create(createCtx, fmt.Sprintf("session-resume-batch-%d", i), "resume me", "", "")
		if err != nil {
			t.Fatalf("create task %d: %v", i, err)
		}
		if err := taskStore.SetStatus(ctx, task.ID, serverPorts.TaskStatusPending); err != nil {
			t.Fatalf("set pending status for task %d: %v", i, err)
		}
	}

	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
		WithResumeClaimBatchSize(2),
	)

	resumed, err := svc.ResumePendingTasks(ctx)
	if err != nil {
		t.Fatalf("resume pending tasks: %v", err)
	}
	if resumed != 2 {
		t.Fatalf("expected resumed count to be capped to 2, got %d", resumed)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if agentExecutor.executionCount() == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected 2 resumed executions, got %d", agentExecutor.executionCount())
}

func TestTaskExecutionService_PrepareGracefulShutdown(t *testing.T) {
	ctx := context.Background()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	// Create two running tasks and one completed task.
	runCtx1 := id.WithRunID(ctx, "run-shutdown-1")
	task1, err := taskStore.Create(runCtx1, "session-1", "analyze data", "planner", "safe")
	if err != nil {
		t.Fatalf("create task1: %v", err)
	}
	if err := taskStore.SetStatus(ctx, task1.ID, serverPorts.TaskStatusRunning); err != nil {
		t.Fatalf("set running status for task1: %v", err)
	}

	runCtx2 := id.WithRunID(ctx, "run-shutdown-2")
	task2, err := taskStore.Create(runCtx2, "session-2", "generate report", "planner", "safe")
	if err != nil {
		t.Fatalf("create task2: %v", err)
	}
	if err := taskStore.SetStatus(ctx, task2.ID, serverPorts.TaskStatusRunning); err != nil {
		t.Fatalf("set running status for task2: %v", err)
	}

	runCtx3 := id.WithRunID(ctx, "run-shutdown-completed")
	task3, err := taskStore.Create(runCtx3, "session-3", "already done", "planner", "safe")
	if err != nil {
		t.Fatalf("create task3: %v", err)
	}
	if err := taskStore.SetStatus(ctx, task3.ID, serverPorts.TaskStatusCompleted); err != nil {
		t.Fatalf("set completed status for task3: %v", err)
	}

	sessionStore := NewMockSessionStore()
	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
	)

	// Simulate in-flight tasks by registering cancel funcs.
	canceled := make(map[string]bool)
	var cancelMu sync.Mutex
	for _, taskID := range []string{task1.ID, task2.ID, task3.ID} {
		tid := taskID
		_, cancelFn := context.WithCancelCause(ctx)
		svc.cancelMu.Lock()
		svc.cancelFuncs[tid] = func(cause error) {
			cancelMu.Lock()
			canceled[tid] = true
			cancelMu.Unlock()
			cancelFn(cause)
		}
		svc.cancelMu.Unlock()
	}

	interrupted := svc.PrepareGracefulShutdown(ctx)

	// All three cancel funcs should have been invoked.
	cancelMu.Lock()
	if len(canceled) != 3 {
		cancelMu.Unlock()
		t.Fatalf("expected 3 cancelled tasks, got %d", len(canceled))
	}
	cancelMu.Unlock()

	// Only the two running tasks should be in the interrupted list.
	if len(interrupted) != 2 {
		t.Fatalf("expected 2 interrupted tasks, got %d", len(interrupted))
	}

	interruptedIDs := map[string]string{}
	for _, it := range interrupted {
		interruptedIDs[it.ID] = it.Description
	}
	if interruptedIDs[task1.ID] != "analyze data" {
		t.Fatalf("expected task1 description 'analyze data', got %q", interruptedIDs[task1.ID])
	}
	if interruptedIDs[task2.ID] != "generate report" {
		t.Fatalf("expected task2 description 'generate report', got %q", interruptedIDs[task2.ID])
	}

	// Running tasks should now be pending.
	got1, _ := taskStore.Get(ctx, task1.ID)
	if got1.Status != serverPorts.TaskStatusPending {
		t.Fatalf("expected task1 status pending, got %s", got1.Status)
	}
	got2, _ := taskStore.Get(ctx, task2.ID)
	if got2.Status != serverPorts.TaskStatusPending {
		t.Fatalf("expected task2 status pending, got %s", got2.Status)
	}

	// Completed task should remain completed.
	got3, _ := taskStore.Get(ctx, task3.ID)
	if got3.Status != serverPorts.TaskStatusCompleted {
		t.Fatalf("expected task3 status completed, got %s", got3.Status)
	}

	// Cancel funcs map should be cleared.
	svc.cancelMu.RLock()
	if len(svc.cancelFuncs) != 0 {
		svc.cancelMu.RUnlock()
		t.Fatalf("expected cancel funcs to be cleared, got %d", len(svc.cancelFuncs))
	}
	svc.cancelMu.RUnlock()
}

func TestTaskExecutionService_PrepareGracefulShutdown_NoRunningTasks(t *testing.T) {
	ctx := context.Background()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	sessionStore := NewMockSessionStore()
	agentExecutor := &resumeAgentExecutor{sessionStore: sessionStore}
	svc := NewTaskExecutionService(
		agentExecutor,
		NewEventBroadcaster(),
		taskStore,
	)

	interrupted := svc.PrepareGracefulShutdown(ctx)
	if interrupted != nil {
		t.Fatalf("expected nil interrupted list, got %v", interrupted)
	}
}
