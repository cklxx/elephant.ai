package lark

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestCreateTask_SummaryOnly(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := string(readBody(r))
		if strings.Contains(body, "description") {
			t.Error("description should not be sent when empty")
		}
		task := taskJSON("t-1", "Buy milk", "")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	got, err := client.Task().CreateTask(context.Background(), CreateTaskRequest{
		Summary: "Buy milk",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Summary != "Buy milk" {
		t.Errorf("Summary = %q, want %q", got.Summary, "Buy milk")
	}
	if got.Description != "" {
		t.Errorf("Description = %q, want empty", got.Description)
	}
}

func TestCreateTask_WithDescription(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := string(readBody(r))
		if !strings.Contains(body, "description") {
			t.Error("expected description in request body")
		}
		task := taskJSONFull("t-2", "Refactor auth", "Move JWT logic to shared package", "")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	got, err := client.Task().CreateTask(context.Background(), CreateTaskRequest{
		Summary:     "Refactor auth",
		Description: "Move JWT logic to shared package",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Summary != "Refactor auth" {
		t.Errorf("Summary = %q, want %q", got.Summary, "Refactor auth")
	}
	if got.Description != "Move JWT logic to shared package" {
		t.Errorf("Description = %q, want %q", got.Description, "Move JWT logic to shared package")
	}
}

func TestPatchTask_UpdateDescription(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := string(readBody(r))
		if !strings.Contains(body, `"description"`) {
			t.Error("expected description in update_fields")
		}
		task := taskJSONFull("t-3", "Original title", "Updated body text", "")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{"task": task}))
	})
	defer srv.Close()

	desc := "Updated body text"
	got, err := client.Task().PatchTask(context.Background(), PatchTaskRequest{
		TaskID:      "t-3",
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Description != "Updated body text" {
		t.Errorf("Description = %q, want %q", got.Description, "Updated body text")
	}
}

func TestParseLarkTask_IncludesDescription(t *testing.T) {
	// Verify that parseLarkTask correctly reads the description field.
	summary := "Task title"
	desc := "Task details here"
	guid := "guid-1"

	item := mockTask(guid, summary, desc)
	got := parseLarkTask(item)

	if got.TaskID != guid {
		t.Errorf("TaskID = %q, want %q", got.TaskID, guid)
	}
	if got.Summary != summary {
		t.Errorf("Summary = %q, want %q", got.Summary, summary)
	}
	if got.Description != desc {
		t.Errorf("Description = %q, want %q", got.Description, desc)
	}
}

func TestParseLarkTask_NilDescription(t *testing.T) {
	summary := "Task title"
	guid := "guid-2"

	item := mockTask(guid, summary, "")
	item.Description = nil // explicitly nil
	got := parseLarkTask(item)

	if got.Description != "" {
		t.Errorf("Description = %q, want empty for nil", got.Description)
	}
}
