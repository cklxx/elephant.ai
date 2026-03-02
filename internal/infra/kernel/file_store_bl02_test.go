package kernel

import (
	"os"
	"path/filepath"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
)

// TestFileStore_Load_EmptyFileIsError verifies BL-02: an existing but empty
// dispatch store file (e.g. after a truncated AtomicWrite) is treated as
// corrupt rather than silently discarding all dispatch records.
func TestFileStore_Load_EmptyFileIsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dispatch_store.json")

	// Write an empty file (simulate mid-crash truncated write).
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	fs := &FileStore{
		filePath:   path,
		dispatches: make(map[string]kerneldomain.Dispatch),
	}
	err := fs.load()
	if err == nil {
		t.Fatal("expected error for empty-but-existing store file, got nil")
	}
	t.Logf("got expected error: %v", err)
}
