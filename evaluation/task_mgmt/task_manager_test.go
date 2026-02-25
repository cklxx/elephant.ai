package task_mgmt

import (
	"strings"
	"testing"
)

func newTestManager(t *testing.T) *TaskManager {
	t.Helper()
	store, err := NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewTaskManager(store)
}

func TestTaskManager_Create(t *testing.T) {
	mgr := newTestManager(t)

	task, err := mgr.Create(CreateTaskRequest{
		Name:        "Test Eval",
		Description: "Run evaluation tests",
		DatasetPath: "./data/test.json",
		Config:      TaskConfig{InstanceLimit: 5, MaxWorkers: 2, EnableMetrics: true},
		Tags:        []string{"regression", "nightly"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if !strings.HasPrefix(task.ID, "evtask_") {
		t.Errorf("unexpected ID format: %s", task.ID)
	}
	if task.Name != "Test Eval" {
		t.Errorf("unexpected name: %s", task.Name)
	}
	if task.Status != TaskStatusActive {
		t.Errorf("expected active status, got %s", task.Status)
	}
}

func TestTaskManager_CreateEmptyName(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.Create(CreateTaskRequest{Name: "  "})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestTaskManager_GetAndList(t *testing.T) {
	mgr := newTestManager(t)

	task1, err := mgr.Create(CreateTaskRequest{Name: "Task 1"})
	if err != nil {
		t.Fatalf("create task1: %v", err)
	}
	if _, err := mgr.Create(CreateTaskRequest{Name: "Task 2"}); err != nil {
		t.Fatalf("create task2: %v", err)
	}

	got, err := mgr.Get(task1.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Task 1" {
		t.Errorf("expected 'Task 1', got %q", got.Name)
	}

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(list))
	}
}

func TestTaskManager_Update(t *testing.T) {
	mgr := newTestManager(t)

	task, err := mgr.Create(CreateTaskRequest{
		Name:   "Original",
		Config: TaskConfig{InstanceLimit: 5},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "Updated Name"
	newStatus := TaskStatusArchived
	updated, err := mgr.Update(task.ID, UpdateTaskRequest{
		Name:   &newName,
		Status: &newStatus,
		Config: &TaskConfig{InstanceLimit: 20, MaxWorkers: 4},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}
	if updated.Status != TaskStatusArchived {
		t.Errorf("expected archived, got %s", updated.Status)
	}
	if updated.Config.InstanceLimit != 20 {
		t.Errorf("expected instance limit 20, got %d", updated.Config.InstanceLimit)
	}
}

func TestTaskManager_UpdateNonexistent(t *testing.T) {
	mgr := newTestManager(t)
	name := "x"
	_, err := mgr.Update("nonexistent", UpdateTaskRequest{Name: &name})
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestTaskManager_Delete(t *testing.T) {
	mgr := newTestManager(t)

	task, err := mgr.Create(CreateTaskRequest{Name: "To Delete"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := mgr.Delete(task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = mgr.Get(task.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestTaskManager_RecordRun(t *testing.T) {
	mgr := newTestManager(t)

	task, err := mgr.Create(CreateTaskRequest{Name: "Runner"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	run, err := mgr.RecordRun(task.ID, "eval-job-123")
	if err != nil {
		t.Fatalf("record run: %v", err)
	}

	if run.TaskID != task.ID {
		t.Errorf("unexpected task ID: %s", run.TaskID)
	}
	if run.EvalJobID != "eval-job-123" {
		t.Errorf("unexpected eval job ID: %s", run.EvalJobID)
	}
	if run.Status != RunStatusPending {
		t.Errorf("expected pending, got %s", run.Status)
	}

	runs, err := mgr.ListRuns(task.ID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run, got %d", len(runs))
	}
}

func TestTaskManager_RecordRunNonexistentTask(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.RecordRun("nonexistent", "job-1")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestTaskManager_UpdateMetadataMerge(t *testing.T) {
	mgr := newTestManager(t)

	task, err := mgr.Create(CreateTaskRequest{
		Name:     "Metadata Test",
		Metadata: map[string]string{"key1": "val1"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := mgr.Update(task.ID, UpdateTaskRequest{
		Metadata: map[string]string{"key2": "val2"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Metadata["key1"] != "val1" {
		t.Errorf("key1 should be preserved")
	}
	if updated.Metadata["key2"] != "val2" {
		t.Errorf("key2 should be added")
	}
}
