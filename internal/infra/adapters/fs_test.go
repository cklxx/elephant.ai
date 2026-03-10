package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileEventAppender_AppendLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.log")

	a := NewFileEventAppender()
	a.AppendLine(path, "line1")
	a.AppendLine(path, "  line2  ")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("expected line1, got %s", lines[0])
	}
	if lines[1] != "line2" {
		t.Errorf("expected line2 (trimmed), got %s", lines[1])
	}
}

func TestFileEventAppender_EmptyPath(t *testing.T) {
	a := NewFileEventAppender()
	a.AppendLine("", "data")
	a.AppendLine("   ", "data")
}

func TestFileEventAppender_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "events.log")

	a := NewFileEventAppender()
	a.AppendLine(path, "hello")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "hello" {
		t.Errorf("expected hello, got %s", string(data))
	}
}

func TestOSAtomicWriter_WriteFileAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	w := NewOSAtomicWriter()
	err := w.WriteFileAtomically(path, []byte("hello world"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected hello world, got %s", string(data))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("expected 0644, got %o", info.Mode().Perm())
	}
}

func TestOSAtomicWriter_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "file.txt")

	w := NewOSAtomicWriter()
	err := w.WriteFileAtomically(path, []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Errorf("expected data, got %s", string(data))
	}
}

func TestOSAtomicWriter_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	w := NewOSAtomicWriter()
	_ = w.WriteFileAtomically(path, []byte("first"), 0o644)
	_ = w.WriteFileAtomically(path, []byte("second"), 0o644)

	data, _ := os.ReadFile(path)
	if string(data) != "second" {
		t.Errorf("expected second, got %s", string(data))
	}
}
