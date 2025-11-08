package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/storage/craftsync"
	id "alex/internal/utils/id"
)

func TestCraftServiceDeleteRemovesMirror(t *testing.T) {
	store := newMemorySessionStore()
	blob := newMemoryBlobStore()

	ctx := id.WithUserID(context.Background(), "craft-user")
	session, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create session failed: %v", err)
	}

	artifact := ports.Artifact{
		ID:         id.NewArtifactID(),
		SessionID:  session.ID,
		UserID:     session.UserID,
		Name:       "draft.html",
		MediaType:  "text/html",
		StorageKey: "users/craft-user/draft",
		Source:     "workbench-article",
		CreatedAt:  time.Now(),
	}
	session.Artifacts = []ports.Artifact{artifact}
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save session failed: %v", err)
	}

	blob.objects[artifact.StorageKey] = []byte("content")

	mirrorDir := t.TempDir()
	mirror, err := craftsync.NewFilesystemMirror(mirrorDir)
	if err != nil {
		t.Fatalf("NewFilesystemMirror returned error: %v", err)
	}

	meta := craftsync.ArtifactMetadata{
		ID:         artifact.ID,
		UserID:     artifact.UserID,
		SessionID:  artifact.SessionID,
		Name:       artifact.Name,
		MediaType:  artifact.MediaType,
		StorageKey: artifact.StorageKey,
		Source:     artifact.Source,
		CreatedAt:  artifact.CreatedAt,
	}
	if _, err := mirror.Mirror(ctx, meta, []byte("content")); err != nil {
		t.Fatalf("mirror pre-populate failed: %v", err)
	}

	service := NewCraftService(store, blob, mirror)
	if err := service.Delete(ctx, artifact.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	pattern := filepath.Join(mirrorDir, "*", "*", artifact.ID, "metadata.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected mirrored metadata to be removed, found %d entries", len(matches))
	}
	if _, exists := blob.objects[artifact.StorageKey]; exists {
		t.Fatalf("expected blob object %s to be deleted", artifact.StorageKey)
	}
}
