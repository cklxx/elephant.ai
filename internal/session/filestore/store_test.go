package filestore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

func TestStore_SavePersistsParentTaskMetadata(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	ctx := id.WithUserID(context.Background(), "user-test")
	session, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session.Metadata["session_id"] = session.ID
	session.Metadata["last_task_id"] = "task-123"
	session.Metadata["last_parent_task_id"] = "parent-456"

	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Use a fresh store to ensure data round-trips through disk
	reloadedStore := New(baseDir)
	reloaded, err := reloadedStore.Get(ctx, session.ID)
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

	ctx := id.WithUserID(context.Background(), "user-test")
	session, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session.Metadata["last_parent_task_id"] = "parent-789"
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got := reloaded.Metadata["last_parent_task_id"]; got != "parent-789" {
		t.Fatalf("expected last_parent_task_id %q, got %q", "parent-789", got)
	}
}

func TestStore_ListAdoptsLegacySessions(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	legacy := ports.Session{
		ID:       "session-legacy",
		Messages: []ports.Message{},
		Todos:    []ports.Todo{},
		Metadata: map[string]string{},
		Artifacts: []ports.Artifact{{
			ID:        "artifact-1",
			Name:      "report.txt",
			MediaType: "text/plain",
			CreatedAt: time.Now(),
		}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	payload, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal legacy session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, legacy.ID+".json"), payload, 0o644); err != nil {
		t.Fatalf("failed to write legacy session file: %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-1")
	ids, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != legacy.ID {
		t.Fatalf("expected legacy session id to be adopted, got %v", ids)
	}

	adopted, err := store.Get(ctx, legacy.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if adopted.UserID != "user-1" {
		t.Fatalf("expected adopted session user_id to be 'user-1', got %q", adopted.UserID)
	}
	if len(adopted.Artifacts) != 1 {
		t.Fatalf("expected artifacts to round-trip, got %d", len(adopted.Artifacts))
	}
	artifact := adopted.Artifacts[0]
	if artifact.UserID != "user-1" {
		t.Fatalf("expected artifact user_id to be updated, got %q", artifact.UserID)
	}
	if artifact.SessionID != legacy.ID {
		t.Fatalf("expected artifact session_id to be updated, got %q", artifact.SessionID)
	}

	otherCtx := id.WithUserID(context.Background(), "user-2")
	otherIDs, err := store.List(otherCtx)
	if err != nil {
		t.Fatalf("List() with other user error = %v", err)
	}
	if len(otherIDs) != 0 {
		t.Fatalf("expected other user to see no sessions, got %v", otherIDs)
	}
}

func TestStore_GetAdoptsLegacySession(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	legacy := ports.Session{
		ID:        "session-get",
		Messages:  []ports.Message{},
		Todos:     []ports.Todo{},
		Metadata:  map[string]string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	payload, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal legacy session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, legacy.ID+".json"), payload, 0o644); err != nil {
		t.Fatalf("failed to write legacy session file: %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-42")
	adopted, err := store.Get(ctx, legacy.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if adopted.UserID != "user-42" {
		t.Fatalf("expected adopted session user_id to be 'user-42', got %q", adopted.UserID)
	}

	// Ensure the user_id persisted to disk
	storeReloaded := New(baseDir)
	reloaded, err := storeReloaded.Get(ctx, legacy.ID)
	if err != nil {
		t.Fatalf("Get() after adoption error = %v", err)
	}
	if reloaded.UserID != "user-42" {
		t.Fatalf("expected persisted user_id to remain 'user-42', got %q", reloaded.UserID)
	}
}
