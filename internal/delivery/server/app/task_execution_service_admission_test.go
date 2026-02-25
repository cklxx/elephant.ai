package app

import (
	"context"
	"errors"
	"testing"
	"time"

	serverPorts "alex/internal/delivery/server/ports"
	sessionstate "alex/internal/infra/session/state_store"
)

func TestTaskExecutionService_ExecuteTaskAsync_AdmissionTimeout(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	defer taskStore.Close()

	svc := NewTaskExecutionService(
		NewMockAgentCoordinator(sessionStore),
		NewEventBroadcaster(),
		taskStore,
		WithTaskStateStore(sessionstate.NewInMemoryStore()),
		WithTaskOwnerID("owner-a"),
		WithTaskAdmissionLimit(1),
	)

	svc.admissionSem <- struct{}{}
	defer func() { <-svc.admissionSem }()

	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	task, err := svc.ExecuteTaskAsync(timeoutCtx, "admission timeout task", "", "", "")
	if err == nil {
		t.Fatal("expected admission timeout error")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if task == nil || task.ID == "" {
		t.Fatal("expected task record to be returned on admission timeout")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, getErr := taskStore.Get(ctx, task.ID)
		if getErr != nil {
			t.Fatalf("get task: %v", getErr)
		}
		if got.Status == serverPorts.TaskStatusFailed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	got, getErr := taskStore.Get(ctx, task.ID)
	if getErr != nil {
		t.Fatalf("get task final: %v", getErr)
	}
	if got.Status != serverPorts.TaskStatusFailed {
		t.Fatalf("expected task status failed after admission timeout, got %s", got.Status)
	}

	svc.cancelMu.RLock()
	_, exists := svc.cancelFuncs[task.ID]
	svc.cancelMu.RUnlock()
	if exists {
		t.Fatal("expected no cancel function registered when admission failed")
	}
}
