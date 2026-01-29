package filestore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	storage "alex/internal/agent/ports/storage"
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

func TestStore_AttachmentsStoredSeparatelyAndSanitized(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	session, err := store.Create(context.Background())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session.Attachments = map[string]ports.Attachment{
		"safe.png": {
			Name:      "safe.png",
			MediaType: "image/png",
			URI:       "https://example.com/safe.png",
			Data:      "base64payload",
		},
		"inline.png": {
			Name:      "inline.png",
			MediaType: "image/png",
			URI:       "data:image/png;base64,AAAA",
			Data:      "AAAA",
		},
	}

	if err := store.Save(context.Background(), session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	mainPath := filepath.Join(baseDir, session.ID+".json")
	mainBytes, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main session file: %v", err)
	}
	if strings.Contains(string(mainBytes), "attachments") {
		t.Fatalf("expected main session file to exclude attachments, got: %s", string(mainBytes))
	}

	attPath := filepath.Join(baseDir, session.ID+"_attachments.json")
	attBytes, err := os.ReadFile(attPath)
	if err != nil {
		t.Fatalf("read attachments file: %v", err)
	}

	var attachments map[string]ports.Attachment
	if err := json.Unmarshal(attBytes, &attachments); err != nil {
		t.Fatalf("decode attachments: %v", err)
	}

	if len(attachments) != 1 {
		t.Fatalf("expected only sanitized attachments to persist, got %d", len(attachments))
	}
	safe := attachments["safe.png"]
	if safe.Data != "" {
		t.Fatalf("expected attachment data to be stripped, got %q", safe.Data)
	}
	if safe.URI == "" || strings.HasPrefix(strings.ToLower(safe.URI), "data:") {
		t.Fatalf("expected safe URI, got %q", safe.URI)
	}

	reloaded, err := store.Get(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(reloaded.Attachments) != 1 {
		t.Fatalf("expected 1 attachment after reload, got %d", len(reloaded.Attachments))
	}
	if _, ok := reloaded.Attachments["safe.png"]; !ok {
		t.Fatalf("expected safe.png attachment to round-trip")
	}
}

func TestStore_RejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	if _, err := store.Get(context.Background(), "../escape"); err == nil {
		t.Fatalf("Get() expected invalid session ID error")
	}

	session := &storage.Session{
		ID:       "../escape",
		Metadata: map[string]string{},
	}
	if err := store.Save(context.Background(), session); err == nil {
		t.Fatalf("Save() expected invalid session ID error")
	}

	if err := store.Delete(context.Background(), "../escape"); err == nil {
		t.Fatalf("Delete() expected invalid session ID error")
	}
}

func TestStore_GetMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	_, err := store.Get(context.Background(), "missing-session")
	if !errors.Is(err, storage.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}
