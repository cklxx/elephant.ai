package craftsync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type metadataFile struct {
	LocalFilename string `json:"local_filename"`
	Name          string `json:"name"`
	MediaType     string `json:"media_type"`
	UserID        string `json:"user_id"`
	SessionID     string `json:"session_id"`
}

func TestFilesystemMirrorWritesContentAndMetadata(t *testing.T) {
	baseDir := t.TempDir()
	mirror, err := NewFilesystemMirror(baseDir)
	if err != nil {
		t.Fatalf("NewFilesystemMirror returned error: %v", err)
	}

	meta := ArtifactMetadata{
		ID:        "craft-1",
		UserID:    "user A",
		SessionID: "session/1",
		Name:      "文章草稿",
		MediaType: "text/html",
		CreatedAt: time.Now(),
	}

	path, err := mirror.Mirror(context.Background(), meta, []byte("<html><body>hello</body></html>"))
	if err != nil {
		t.Fatalf("Mirror returned error: %v", err)
	}
	if path == "" {
		t.Fatalf("expected content path to be returned")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected content file to exist: %v", err)
	}

	metadataPath := filepath.Join(baseDir, "user-A", "session-1", "craft-1", "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}

	var payload metadataFile
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}
	if payload.LocalFilename == "" {
		t.Fatalf("expected local filename to be recorded")
	}
	if payload.Name != meta.Name {
		t.Fatalf("metadata name mismatch: %s", payload.Name)
	}
	if payload.MediaType != meta.MediaType {
		t.Fatalf("metadata media type mismatch: %s", payload.MediaType)
	}
	if payload.UserID != "user A" {
		t.Fatalf("expected user id preserved, got %s", payload.UserID)
	}
	if payload.SessionID != "session/1" {
		t.Fatalf("expected session id preserved, got %s", payload.SessionID)
	}
}

func TestFilesystemMirrorRemoveCleansDirectory(t *testing.T) {
	baseDir := t.TempDir()
	mirror, err := NewFilesystemMirror(baseDir)
	if err != nil {
		t.Fatalf("NewFilesystemMirror returned error: %v", err)
	}

	meta := ArtifactMetadata{ID: "craft-2", UserID: "user/B", SessionID: "sess"}
	if _, err := mirror.Mirror(context.Background(), meta, []byte("content")); err != nil {
		t.Fatalf("Mirror returned error: %v", err)
	}

	targetDir := filepath.Join(baseDir, "user-B", "sess", "craft-2")
	if _, err := os.Stat(targetDir); err != nil {
		t.Fatalf("expected directory to exist: %v", err)
	}

	if err := mirror.Remove(context.Background(), meta); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Fatalf("expected directory to be removed, got err=%v", err)
	}
}

func TestFilesystemMirrorSkipsContentWhenEmpty(t *testing.T) {
	baseDir := t.TempDir()
	mirror, err := NewFilesystemMirror(baseDir)
	if err != nil {
		t.Fatalf("NewFilesystemMirror returned error: %v", err)
	}

	meta := ArtifactMetadata{ID: "craft-3", UserID: "user", SessionID: "sess", MediaType: "application/json", CreatedAt: time.Now()}
	if _, err := mirror.Mirror(context.Background(), meta, nil); err != nil {
		t.Fatalf("Mirror returned error: %v", err)
	}

	metadataPath := filepath.Join(baseDir, "user", "sess", "craft-3", "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	var payload metadataFile
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}
	if payload.LocalFilename != "" {
		t.Fatalf("expected empty local filename for nil content, got %s", payload.LocalFilename)
	}
}
