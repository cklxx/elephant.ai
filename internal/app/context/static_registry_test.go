package context

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStaticRegistryVersionStableAcrossReloads(t *testing.T) {
	root := buildStaticContextTree(t)
	registry := newStaticRegistry(root, time.Millisecond, nil, nil)

	ctx := context.Background()
	snap1, err := registry.currentSnapshot(ctx)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}

	registry.expires = time.Now().Add(-time.Minute)

	snap2, err := registry.currentSnapshot(ctx)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}

	if snap1.Version == "" {
		t.Fatalf("expected non-empty version")
	}
	if snap1.Version != snap2.Version {
		t.Fatalf("expected version to remain stable across reloads, got %q vs %q", snap1.Version, snap2.Version)
	}
}

func TestStaticRegistryVersionChangesWhenFilesChange(t *testing.T) {
	root := buildStaticContextTree(t)
	registry := newStaticRegistry(root, time.Millisecond, nil, nil)

	ctx := context.Background()
	snap1, err := registry.currentSnapshot(ctx)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}

	personaPath := filepath.Join(root, "personas", "default.yaml")
	if err := os.WriteFile(personaPath, []byte("id: default\ntone: bold"), 0o644); err != nil {
		t.Fatalf("update persona: %v", err)
	}

	registry.expires = time.Now().Add(-time.Minute)

	snap2, err := registry.currentSnapshot(ctx)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}

	if snap1.Version == snap2.Version {
		t.Fatalf("expected version to change after modifying static config")
	}
}
