package filestore

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_SavePersistsParentTaskMetadata(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	session, err := store.Create(context.Background())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session.Metadata["session_id"] = session.ID
	session.Metadata["last_task_id"] = "task-123"
	session.Metadata["last_parent_task_id"] = "parent-456"

	if err := store.Save(context.Background(), session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Use a fresh store to ensure data round-trips through disk
	reloadedStore := New(baseDir)
	reloaded, err := reloadedStore.Get(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got := reloaded.Metadata["last_parent_task_id"]; got != "parent-456" {
		t.Fatalf("expected last_parent_task_id to round-trip, got %q", got)
	}
}

func TestStore_SaveCreatesFilesRelativeToBaseDir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(filepath.Join(baseDir, "sessions"))

	session, err := store.Create(context.Background())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session.Metadata["last_parent_task_id"] = "parent-789"
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := store.Get(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got := reloaded.Metadata["last_parent_task_id"]; got != "parent-789" {
		t.Fatalf("expected last_parent_task_id %q, got %q", "parent-789", got)
	}
}
