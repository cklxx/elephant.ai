package coordinator

import (
	"context"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
)

func TestBackgroundTaskRegistryCleanupReclaimsIdleTerminalManagers(t *testing.T) {
	now := time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)
	registry := newBackgroundTaskRegistry()
	registry.cleanupInterval = time.Minute
	registry.idleTTL = 2 * time.Minute
	registry.nowFn = func() time.Time { return now }

	shutdownCalls := 0
	registry.shutdownFn = func(_ *react.BackgroundTaskManager) {
		shutdownCalls++
	}

	registry.Get("session-a", func() *react.BackgroundTaskManager {
		return newTestBackgroundManager("session-a", func(context.Context, string, string, agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "ok"}, nil
		})
	})

	now = now.Add(3 * time.Minute)
	_ = registry.Get("session-b", nil) // trigger cleanup

	if _, ok := registry.managers["session-a"]; ok {
		t.Fatalf("expected idle terminal manager to be reclaimed")
	}
	if shutdownCalls != 1 {
		t.Fatalf("expected shutdown called once, got %d", shutdownCalls)
	}
}

func TestBackgroundTaskRegistryCleanupKeepsNonTerminalManagers(t *testing.T) {
	now := time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)
	registry := newBackgroundTaskRegistry()
	registry.cleanupInterval = time.Minute
	registry.idleTTL = 2 * time.Minute
	registry.nowFn = func() time.Time { return now }

	shutdownCalls := 0
	registry.shutdownFn = func(_ *react.BackgroundTaskManager) {
		shutdownCalls++
	}

	unblock := make(chan struct{})
	runningManager := newTestBackgroundManager(
		"session-running",
		func(ctx context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-unblock:
				return &agent.TaskResult{Answer: "done"}, nil
			}
		},
	)
	t.Cleanup(func() {
		close(unblock)
		runningManager.Shutdown()
	})
	if err := runningManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      "task-1",
		Description: "long task",
		Prompt:      "run",
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	registry.Get("session-running", func() *react.BackgroundTaskManager {
		return runningManager
	})

	now = now.Add(3 * time.Minute)
	_ = registry.Get("session-other", nil) // trigger cleanup

	if _, ok := registry.managers["session-running"]; !ok {
		t.Fatalf("expected non-terminal manager to remain registered")
	}
	if shutdownCalls != 0 {
		t.Fatalf("expected no shutdown for non-terminal manager, got %d", shutdownCalls)
	}
}

func TestManagerTasksTerminal(t *testing.T) {
	mgr := newTestBackgroundManager("session-terminal", func(context.Context, string, string, agent.EventListener) (*agent.TaskResult, error) {
		return &agent.TaskResult{Answer: "done"}, nil
	})
	defer mgr.Shutdown()

	if !managerTasksTerminal(mgr) {
		t.Fatalf("expected manager without tasks to be terminal")
	}

	if err := mgr.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      "task-1",
		Description: "task",
		Prompt:      "run",
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !mgr.AwaitAll(2 * time.Second) {
		t.Fatalf("expected task to complete")
	}
	if !managerTasksTerminal(mgr) {
		t.Fatalf("expected manager with completed tasks to be terminal")
	}
}

func newTestBackgroundManager(
	sessionID string,
	executeTask func(context.Context, string, string, agent.EventListener) (*agent.TaskResult, error),
) *react.BackgroundTaskManager {
	return react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		GoRunner: func(_ agent.Logger, _ string, fn func()) {
			go fn()
		},
		ExecuteTask: executeTask,
		SessionID:   sessionID,
	})
}
