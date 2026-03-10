package bootstrap

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"alex/internal/app/di"
	"alex/internal/delivery/taskadapters"
	taskdomain "alex/internal/domain/task"
)

type stubUnifiedTaskStore struct{}

func (stubUnifiedTaskStore) EnsureSchema(context.Context) error             { return nil }
func (stubUnifiedTaskStore) Create(context.Context, *taskdomain.Task) error { return nil }
func (stubUnifiedTaskStore) Get(context.Context, string) (*taskdomain.Task, error) {
	return &taskdomain.Task{}, nil
}
func (stubUnifiedTaskStore) SetStatus(context.Context, string, taskdomain.Status, ...taskdomain.TransitionOption) error {
	return nil
}
func (stubUnifiedTaskStore) UpdateProgress(context.Context, string, int, int, float64) error {
	return nil
}
func (stubUnifiedTaskStore) SetResult(context.Context, string, string, json.RawMessage, int) error {
	return nil
}
func (stubUnifiedTaskStore) SetError(context.Context, string, string) error { return nil }
func (stubUnifiedTaskStore) SetBridgeMeta(context.Context, string, taskdomain.BridgeMeta) error {
	return nil
}
func (stubUnifiedTaskStore) Delete(context.Context, string) error { return nil }
func (stubUnifiedTaskStore) TryClaimTask(context.Context, string, string, time.Time) (bool, error) {
	return true, nil
}
func (stubUnifiedTaskStore) ClaimResumableTasks(context.Context, string, time.Time, int, ...taskdomain.Status) ([]*taskdomain.Task, error) {
	return nil, nil
}
func (stubUnifiedTaskStore) RenewTaskLease(context.Context, string, string, time.Time) (bool, error) {
	return true, nil
}
func (stubUnifiedTaskStore) ReleaseTaskLease(context.Context, string, string) error { return nil }
func (stubUnifiedTaskStore) ListBySession(context.Context, string, int) ([]*taskdomain.Task, error) {
	return nil, nil
}
func (stubUnifiedTaskStore) ListByChat(context.Context, string, bool, int) ([]*taskdomain.Task, error) {
	return nil, nil
}
func (stubUnifiedTaskStore) ListByStatus(context.Context, ...taskdomain.Status) ([]*taskdomain.Task, error) {
	return nil, nil
}
func (stubUnifiedTaskStore) ListActive(context.Context) ([]*taskdomain.Task, error) { return nil, nil }
func (stubUnifiedTaskStore) List(context.Context, int, int) ([]*taskdomain.Task, int, error) {
	return nil, 0, nil
}
func (stubUnifiedTaskStore) Transitions(context.Context, string) ([]taskdomain.Transition, error) {
	return nil, nil
}
func (stubUnifiedTaskStore) MarkStaleRunning(context.Context, string) error { return nil }
func (stubUnifiedTaskStore) DeleteExpired(context.Context, time.Time) error { return nil }

func TestServerTaskStoreForContainer_UsesUnifiedAdapter(t *testing.T) {
	t.Parallel()

	taskStore, err := serverTaskStoreForContainer(&di.Container{TaskStore: stubUnifiedTaskStore{}})
	if err != nil {
		t.Fatalf("serverTaskStoreForContainer() error = %v", err)
	}
	if _, ok := taskStore.(*taskadapters.ServerAdapter); !ok {
		t.Fatalf("task store type = %T, want *taskadapters.ServerAdapter", taskStore)
	}
}

func TestServerTaskStoreForContainer_ErrorsWithoutUnifiedStore(t *testing.T) {
	t.Parallel()

	if _, err := serverTaskStoreForContainer(&di.Container{}); err == nil {
		t.Fatal("serverTaskStoreForContainer() error = nil, want error")
	}
}

func TestLarkTaskStoreForContainer_UsesUnifiedAdapter(t *testing.T) {
	t.Parallel()

	taskStore, err := larkTaskStoreForContainer(&di.Container{TaskStore: stubUnifiedTaskStore{}})
	if err != nil {
		t.Fatalf("larkTaskStoreForContainer() error = %v", err)
	}
	if _, ok := taskStore.(*taskadapters.LarkAdapter); !ok {
		t.Fatalf("task store type = %T, want *taskadapters.LarkAdapter", taskStore)
	}
}

func TestLarkTaskStoreForContainer_ErrorsWithoutUnifiedStore(t *testing.T) {
	t.Parallel()

	if _, err := larkTaskStoreForContainer(&di.Container{}); err == nil {
		t.Fatal("larkTaskStoreForContainer() error = nil, want error")
	}
}
