package filestore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite_CreatesFileAndParentDirs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "deep", "file.json")

	if err := AtomicWrite(target, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("unexpected content: %s", data)
	}
}

func TestAtomicWrite_NoTempFileLeftOnSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.json")

	if err := AtomicWrite(target, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("expected .tmp file to be cleaned up")
	}
}

func TestReadFileOrEmpty_MissingReturnsNilNil(t *testing.T) {
	data, err := ReadFileOrEmpty(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data, got: %s", data)
	}
}

func TestReadFileOrEmpty_ExistingReturnsContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.json")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFileOrEmpty(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %s", data)
	}
}

func TestResolvePath_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	tests := []struct {
		input string
		want  string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~", home},
		{"/abs/path", "/abs/path"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ResolvePath(tt.input, "")
		if got != tt.want {
			t.Errorf("ResolvePath(%q, \"\") = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolvePath_DefaultFallback(t *testing.T) {
	got := ResolvePath("", "/default/path")
	if got != "/default/path" {
		t.Errorf("expected /default/path, got %q", got)
	}
}

func TestEnsureDir_CreatesNestedDirs(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	if err := EnsureDir(nested); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestMarshalJSONIndent_TrailingNewline(t *testing.T) {
	data, err := MarshalJSONIndent(map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if data[len(data)-1] != '\n' {
		t.Fatal("expected trailing newline")
	}
}
