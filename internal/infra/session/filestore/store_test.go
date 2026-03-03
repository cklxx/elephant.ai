package filestore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	storage "alex/internal/domain/agent/ports/storage"
)

func TestNew_ExpandsHomePath(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	s, ok := New("~/.alex/sessions").(*store)
	if !ok {
		t.Fatalf("New() did not return *store")
	}

	want := filepath.Join(tempHome, ".alex", "sessions")
	if s.baseDir != want {
		t.Fatalf("baseDir = %q, want %q", s.baseDir, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected session dir to be created: %v", err)
	}
}

func TestNew_ExpandsEnvPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ALEX_SESSION_ROOT", filepath.Join(root, "session-root"))

	s, ok := New("$ALEX_SESSION_ROOT/store").(*store)
	if !ok {
		t.Fatalf("New() did not return *store")
	}

	want := filepath.Join(root, "session-root", "store")
	if s.baseDir != want {
		t.Fatalf("baseDir = %q, want %q", s.baseDir, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected session dir to be created: %v", err)
	}
}

func TestNew_EmptyPathDefaultsToCurrentDirectory(t *testing.T) {
	t.Parallel()

	s, ok := New("").(*store)
	if !ok {
		t.Fatalf("New() did not return *store")
	}
	if s.baseDir != "." {
		t.Fatalf("baseDir = %q, want %q", s.baseDir, ".")
	}
}

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

func TestStore_GetCompanionAttachmentsOverrideLegacyMainFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := New(baseDir)

	session, err := store.Create(context.Background())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	mainPath := filepath.Join(baseDir, session.ID+".json")
	legacyMain := *session
	legacyMain.Attachments = map[string]ports.Attachment{
		"photo.png": {
			Name: "photo.png",
			URI:  "https://legacy.example.com/photo.png",
		},
	}
	mainBytes, err := json.MarshalIndent(legacyMain, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy main file: %v", err)
	}
	if err := os.WriteFile(mainPath, mainBytes, 0o644); err != nil {
		t.Fatalf("write legacy main file: %v", err)
	}

	companionPath := filepath.Join(baseDir, session.ID+"_attachments.json")
	companion := map[string]ports.Attachment{
		"photo.png": {
			Name: "photo.png",
			URI:  "https://cdn.example.com/photo.png",
		},
	}
	companionBytes, err := json.MarshalIndent(companion, "", "  ")
	if err != nil {
		t.Fatalf("marshal companion attachments: %v", err)
	}
	if err := os.WriteFile(companionPath, companionBytes, 0o644); err != nil {
		t.Fatalf("write companion attachments: %v", err)
	}

	reloaded, err := store.Get(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got := reloaded.Attachments["photo.png"].URI; got != "https://cdn.example.com/photo.png" {
		t.Fatalf("expected companion attachment to override legacy value, got %q", got)
	}
}

func TestStore_ListSessionItemsReadsLightweightHeaders(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	storeImpl, ok := New(baseDir).(*store)
	if !ok {
		t.Fatalf("New() did not return *store")
	}

	createdAt := time.Now().Add(-time.Minute).UTC().Round(time.Second)
	updatedAt := time.Now().UTC().Round(time.Second)
	sessionID := "sess-list-light"
	path := filepath.Join(baseDir, sessionID+".json")
	raw := `{
  "id": "` + sessionID + `",
  "metadata": {"title": "Lightweight Title"},
  "created_at": "` + createdAt.Format(time.RFC3339) + `",
  "updated_at": "` + updatedAt.Format(time.RFC3339) + `",
  "messages": "intentionally-not-an-array"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write session fixture: %v", err)
	}

	items, err := storeImpl.ListSessionItems(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("ListSessionItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 session item, got %d", len(items))
	}
	item := items[0]
	if item.ID != sessionID {
		t.Fatalf("expected ID %q, got %q", sessionID, item.ID)
	}
	if item.Title != "Lightweight Title" {
		t.Fatalf("expected title %q, got %q", "Lightweight Title", item.Title)
	}
	if !item.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected created_at %s, got %s", createdAt, item.CreatedAt)
	}
	if !item.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated_at %s, got %s", updatedAt, item.UpdatedAt)
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
