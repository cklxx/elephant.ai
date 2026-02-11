package kernel

import (
	"os"
	"path/filepath"
	"testing"
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
