package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/shared/logging"
	"alex/internal/shared/markdown"
)

func TestStateFile_ReadNonExistent(t *testing.T) {
	sf := NewStateFile(t.TempDir())
	content, err := sf.Read()
	if err != nil {
		t.Fatalf("Read non-existent: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}
}

func TestStateFile_WriteReadRoundTrip(t *testing.T) {
	sf := NewStateFile(t.TempDir())
	want := "# STATE\n## identity\ntest agent\n"
	if err := sf.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch: got %q, want %q", got, want)
	}
}

func TestStateFile_SeedIdempotent(t *testing.T) {
	sf := NewStateFile(t.TempDir())
	seed := "initial state"
	if err := sf.Seed(seed); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	// Second Seed with different content should not overwrite.
	if err := sf.Seed("different content"); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != seed {
		t.Errorf("Seed was not idempotent: got %q, want %q", got, seed)
	}
}

func TestStateFile_InitAndSystemPromptFiles(t *testing.T) {
	sf := NewStateFile(t.TempDir())

	initContent := "# init\nkernel config"
	if err := sf.SeedInit(initContent); err != nil {
		t.Fatalf("SeedInit: %v", err)
	}
	if err := sf.SeedInit("# overwritten"); err != nil {
		t.Fatalf("second SeedInit: %v", err)
	}
	gotInit, err := sf.ReadInit()
	if err != nil {
		t.Fatalf("ReadInit: %v", err)
	}
	if gotInit != initContent {
		t.Fatalf("init content mismatch: got %q want %q", gotInit, initContent)
	}

	systemPrompt := "You are kernel system prompt."
	if err := sf.WriteSystemPrompt(systemPrompt); err != nil {
		t.Fatalf("WriteSystemPrompt: %v", err)
	}
	gotPrompt, err := sf.ReadSystemPrompt()
	if err != nil {
		t.Fatalf("ReadSystemPrompt: %v", err)
	}
	if gotPrompt != systemPrompt {
		t.Fatalf("system prompt mismatch: got %q want %q", gotPrompt, systemPrompt)
	}

	if filepath.Base(sf.InitPath()) != "INIT.md" {
		t.Fatalf("unexpected init path: %s", sf.InitPath())
	}
	if filepath.Base(sf.SystemPromptPath()) != "SYSTEM_PROMPT.md" {
		t.Fatalf("unexpected system prompt path: %s", sf.SystemPromptPath())
	}
}

func TestStateFile_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	sf := NewStateFile(dir)
	if err := sf.Write("content"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// The tmp file should have been cleaned up by rename.
	tmp := sf.Path() + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("tmp file should not exist after write, err=%v", err)
	}
}

func TestStateFile_WriteCreatesDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")
	sf := NewStateFile(nested)
	if err := sf.Write("test"); err != nil {
		t.Fatalf("Write with nested dir: %v", err)
	}
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "test" {
		t.Errorf("got %q, want %q", got, "test")
	}
}

func newTestVersionedStateFile(t *testing.T) *StateFile {
	t.Helper()
	dir := t.TempDir()
	store := markdown.NewVersionedStore(markdown.StoreConfig{
		Dir:        dir,
		AutoCommit: true,
		Logger:     logging.Nop(),
	})
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("store Init: %v", err)
	}
	return NewVersionedStateFile(dir, store)
}

func TestStateFile_VersionedWriteCreatesGitHistory(t *testing.T) {
	sf := newTestVersionedStateFile(t)
	ctx := context.Background()

	// Write two versions of STATE.md.
	if err := sf.Write("v1"); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if err := sf.CommitCycleBoundary(ctx, "cycle-1"); err != nil {
		t.Fatalf("CommitCycleBoundary 1: %v", err)
	}

	if err := sf.Write("v2"); err != nil {
		t.Fatalf("Write v2: %v", err)
	}
	if err := sf.CommitCycleBoundary(ctx, "cycle-2"); err != nil {
		t.Fatalf("CommitCycleBoundary 2: %v", err)
	}

	// Verify current content is v2.
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "v2" {
		t.Errorf("expected v2, got %q", got)
	}

	// Verify git history has 2 commits for STATE.md.
	entries, err := sf.store.History(ctx, stateFileName, 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected >= 2 history entries, got %d", len(entries))
	}
}

func TestStateFile_CommitCycleBoundary_NoOpWithoutStore(t *testing.T) {
	sf := NewStateFile(t.TempDir())
	// Should not error or panic.
	if err := sf.CommitCycleBoundary(context.Background(), "test"); err != nil {
		t.Fatalf("CommitCycleBoundary on legacy StateFile: %v", err)
	}
}

func TestStateFile_VersionedSeedIdempotent(t *testing.T) {
	sf := newTestVersionedStateFile(t)

	if err := sf.Seed("first"); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	if err := sf.Seed("second"); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "first" {
		t.Errorf("Seed not idempotent: got %q, want %q", got, "first")
	}
}
