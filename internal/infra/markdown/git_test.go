package markdown

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/shared/logging"
)

func newTestGit(t *testing.T) *gitOperations {
	t.Helper()
	dir := t.TempDir()
	g := newGitOperations(dir, logging.Nop())
	if err := g.init(context.Background()); err != nil {
		t.Fatalf("git init: %v", err)
	}
	return g
}

func TestGit_InitIdempotent(t *testing.T) {
	dir := t.TempDir()
	g := newGitOperations(dir, logging.Nop())
	ctx := context.Background()

	if err := g.init(ctx); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if !g.isRepo() {
		t.Fatal("expected .git to exist after init")
	}
	// Second init should be a no-op.
	if err := g.init(ctx); err != nil {
		t.Fatalf("second init: %v", err)
	}
}

func TestGit_AddCommitHasChanges(t *testing.T) {
	g := newTestGit(t)
	ctx := context.Background()

	// No changes initially.
	changed, err := g.hasChanges(ctx)
	if err != nil {
		t.Fatalf("hasChanges: %v", err)
	}
	if changed {
		t.Fatal("expected no changes in fresh repo")
	}

	// Write a file and check again.
	if err := os.WriteFile(filepath.Join(g.dir, "test.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	changed, err = g.hasChanges(ctx)
	if err != nil {
		t.Fatalf("hasChanges after write: %v", err)
	}
	if !changed {
		t.Fatal("expected changes after writing a file")
	}

	// Add and commit.
	if err := g.add(ctx, "test.md"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := g.commit(ctx, "initial"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Should be clean now.
	changed, err = g.hasChanges(ctx)
	if err != nil {
		t.Fatalf("hasChanges after commit: %v", err)
	}
	if changed {
		t.Fatal("expected no changes after commit")
	}
}

func TestGit_Log(t *testing.T) {
	g := newTestGit(t)
	ctx := context.Background()

	// Create two commits.
	if err := os.WriteFile(filepath.Join(g.dir, "a.md"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.add(ctx, "a.md"); err != nil {
		t.Fatal(err)
	}
	if err := g.commit(ctx, "first commit"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(g.dir, "a.md"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.add(ctx, "a.md"); err != nil {
		t.Fatal(err)
	}
	if err := g.commit(ctx, "second commit"); err != nil {
		t.Fatal(err)
	}

	entries, err := g.log(ctx, "a.md", 5)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}
	if entries[0].Subject != "second commit" {
		t.Errorf("expected 'second commit', got %q", entries[0].Subject)
	}
	if entries[1].Subject != "first commit" {
		t.Errorf("expected 'first commit', got %q", entries[1].Subject)
	}
	if entries[0].Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestGit_LogEmpty(t *testing.T) {
	g := newTestGit(t)
	entries, err := g.log(context.Background(), "nonexistent.md", 5)
	if err != nil {
		t.Fatalf("log on empty repo: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestGit_IsRepoBeforeInit(t *testing.T) {
	dir := t.TempDir()
	g := newGitOperations(dir, logging.Nop())
	if g.isRepo() {
		t.Fatal("expected false before init")
	}
}
