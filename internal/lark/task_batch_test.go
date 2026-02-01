package lark

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestBatchCreateTasks_AllSuccess(t *testing.T) {
	cnt := &counter{}
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		n := cnt.next()
		w.Header().Set("Content-Type", "application/json")
		task := taskJSON(fmt.Sprintf("task-%d", n), fmt.Sprintf("Task %d", n), "")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	requests := []CreateTaskRequest{
		{Summary: "Task 1"},
		{Summary: "Task 2"},
		{Summary: "Task 3"},
	}

	results, errs := client.Task().BatchCreateTasks(context.Background(), requests)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if len(errs) != 3 {
		t.Fatalf("expected 3 error slots, got %d", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("task[%d]: unexpected error: %v", i, err)
		}
	}
	for i, task := range results {
		if task.TaskID == "" {
			t.Errorf("task[%d]: expected non-empty TaskID", i)
		}
		wantID := fmt.Sprintf("task-%d", i+1)
		if task.TaskID != wantID {
			t.Errorf("task[%d]: TaskID = %q, want %q", i, task.TaskID, wantID)
		}
	}
}

func TestBatchCreateTasks_MixedSuccessFailure(t *testing.T) {
	cnt := &counter{}
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		n := cnt.next()
		w.Header().Set("Content-Type", "application/json")
		if n == 2 {
			w.Write(jsonResponse(400100, "invalid task", nil))
			return
		}
		task := taskJSON(fmt.Sprintf("task-%d", n), fmt.Sprintf("Task %d", n), "")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	requests := []CreateTaskRequest{
		{Summary: "Good Task"},
		{Summary: "Bad Task"},
		{Summary: "Good Task 2"},
	}

	results, errs := client.Task().BatchCreateTasks(context.Background(), requests)

	// First task: success.
	if errs[0] != nil {
		t.Errorf("task[0]: unexpected error: %v", errs[0])
	}
	if results[0].TaskID == "" {
		t.Error("task[0]: expected non-empty TaskID")
	}

	// Second task: failure.
	if errs[1] == nil {
		t.Error("task[1]: expected error, got nil")
	}
	if results[1].TaskID != "" {
		t.Errorf("task[1]: expected empty TaskID on failure, got %q", results[1].TaskID)
	}

	// Third task: success.
	if errs[2] != nil {
		t.Errorf("task[2]: unexpected error: %v", errs[2])
	}
	if results[2].TaskID == "" {
		t.Error("task[2]: expected non-empty TaskID")
	}
}

func TestBatchCreateTasks_Empty(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for empty batch")
	})
	defer srv.Close()

	results, errs := client.Task().BatchCreateTasks(context.Background(), nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestBatchCompleteTasks_AllSuccess(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The Patch response includes the updated task.
		task := taskJSON("any-id", "any-summary", "1706745600000")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	errs := client.Task().BatchCompleteTasks(context.Background(), []string{"task-1", "task-2"})
	if len(errs) != 2 {
		t.Fatalf("expected 2 error slots, got %d", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("task[%d]: unexpected error: %v", i, err)
		}
	}
}

func TestBatchCompleteTasks_MixedSuccessFailure(t *testing.T) {
	cnt := &counter{}
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		n := cnt.next()
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.Write(jsonResponse(404001, "task not found", nil))
			return
		}
		task := taskJSON("any", "any", "1706745600000")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	errs := client.Task().BatchCompleteTasks(context.Background(), []string{"missing-task", "ok-task"})

	if errs[0] == nil {
		t.Error("task[0]: expected error for missing task, got nil")
	}
	if errs[1] != nil {
		t.Errorf("task[1]: unexpected error: %v", errs[1])
	}
}

func TestBatchCompleteTasks_Empty(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for empty batch")
	})
	defer srv.Close()

	errs := client.Task().BatchCompleteTasks(context.Background(), nil)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestBatchCompleteTasks_SetsCompletedAt(t *testing.T) {
	// Verify the PATCH request includes completed_at in update_fields.
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/tasks/") {
			body := string(readBody(r))
			if !strings.Contains(body, "completed_at") {
				t.Errorf("expected body to contain completed_at, got: %s", body)
			}
		}
		task := taskJSON("task-1", "Test", "1706745600000")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	errs := client.Task().BatchCompleteTasks(context.Background(), []string{"task-1"})
	if errs[0] != nil {
		t.Errorf("unexpected error: %v", errs[0])
	}
}

func TestBatchCreateTasks_AllFailure(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(500000, "internal error", nil))
	})
	defer srv.Close()

	requests := []CreateTaskRequest{
		{Summary: "Fail 1"},
		{Summary: "Fail 2"},
	}

	results, errs := client.Task().BatchCreateTasks(context.Background(), requests)

	for i, err := range errs {
		if err == nil {
			t.Errorf("task[%d]: expected error, got nil", i)
		}
	}
	for i, task := range results {
		if task.TaskID != "" {
			t.Errorf("task[%d]: expected zero-value on failure, got TaskID=%q", i, task.TaskID)
		}
	}
}
