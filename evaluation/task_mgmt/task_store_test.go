package task_mgmt

import (
	"testing"
	"time"
)

func TestTaskStore_SaveAndGet(t *testing.T) {
	store, err := NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	task := &EvalTaskDefinition{
		ID:          "task-1",
		Name:        "Test Task",
		Description: "A test task",
		Status:      TaskStatusActive,
		Config:      TaskConfig{InstanceLimit: 10, MaxWorkers: 2},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	got, err := store.GetTask("task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Name != "Test Task" {
		t.Errorf("expected 'Test Task', got %q", got.Name)
	}
	if got.Config.InstanceLimit != 10 {
		t.Errorf("expected instance limit 10, got %d", got.Config.InstanceLimit)
	}
}

func TestTaskStore_GetNotFound(t *testing.T) {
	store, err := NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.GetTask("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestTaskStore_ListTasks(t *testing.T) {
	store, err := NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := store.SaveTask(&EvalTaskDefinition{ID: "older", Name: "Older", CreatedAt: now.Add(-time.Hour), UpdatedAt: now}); err != nil {
		t.Fatalf("save older task: %v", err)
	}
	if err := store.SaveTask(&EvalTaskDefinition{ID: "newer", Name: "Newer", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("save newer task: %v", err)
	}

	tasks, err := store.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	// Newest first
	if tasks[0].ID != "newer" {
		t.Errorf("expected newest first, got %s", tasks[0].ID)
	}
}

func TestTaskStore_DeleteTask(t *testing.T) {
	store, err := NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SaveTask(&EvalTaskDefinition{ID: "to-delete", Name: "Delete Me"}); err != nil {
		t.Fatalf("save task: %v", err)
	}

	if err := store.DeleteTask("to-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = store.GetTask("to-delete")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestTaskStore_DeleteNotFound(t *testing.T) {
	store, err := NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteTask("nonexistent"); err == nil {
		t.Fatal("expected error for deleting nonexistent task")
	}
}

func TestTaskStore_Runs(t *testing.T) {
	store, err := NewTaskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	run1 := &BatchRun{ID: "run-1", TaskID: "task-1", Status: RunStatusCompleted, StartedAt: now.Add(-time.Hour)}
	run2 := &BatchRun{ID: "run-2", TaskID: "task-1", Status: RunStatusRunning, StartedAt: now}
	run3 := &BatchRun{ID: "run-3", TaskID: "task-2", Status: RunStatusPending, StartedAt: now}

	if err := store.SaveRun(run1); err != nil {
		t.Fatalf("save run1: %v", err)
	}
	if err := store.SaveRun(run2); err != nil {
		t.Fatalf("save run2: %v", err)
	}
	if err := store.SaveRun(run3); err != nil {
		t.Fatalf("save run3: %v", err)
	}

	runs, err := store.ListRunsForTask("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs for task-1, got %d", len(runs))
	}
	// Newest first
	if runs[0].ID != "run-2" {
		t.Errorf("expected newest run first, got %s", runs[0].ID)
	}
}
