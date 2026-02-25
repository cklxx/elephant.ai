package pathutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLocalPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "path-guard-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})
	ctx := WithWorkingDir(context.Background(), base)

	resolved, err := ResolveLocalPath(ctx, "note.txt")
	if err != nil {
		t.Fatalf("expected path to resolve, got error: %v", err)
	}
	if !pathWithinBase(base, resolved) {
		t.Fatalf("expected resolved path %q to stay within base %q", resolved, base)
	}

	escaped, err := ResolveLocalPath(ctx, "../escape.txt")
	if err != nil {
		t.Fatalf("expected traversal path to resolve, got error: %v", err)
	}
	expectedEscaped, err := filepath.Abs(filepath.Clean(filepath.Join(base, "..", "escape.txt")))
	if err != nil {
		t.Fatalf("failed to normalize expected escaped path: %v", err)
	}
	if escaped != expectedEscaped {
		t.Fatalf("expected traversal path %q, got %q", expectedEscaped, escaped)
	}

	outside := filepath.Dir(base)
	outsideResolved, err := ResolveLocalPath(ctx, outside)
	if err != nil {
		t.Fatalf("expected absolute path outside base to resolve, got error: %v", err)
	}
	expectedOutside, err := filepath.Abs(filepath.Clean(outside))
	if err != nil {
		t.Fatalf("failed to normalize expected outside path: %v", err)
	}
	if outsideResolved != expectedOutside {
		t.Fatalf("expected outside path %q, got %q", expectedOutside, outsideResolved)
	}
}

func TestResolveLocalPathAllowsSymlinkPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "path-guard-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})
	outside := t.TempDir()

	link := filepath.Join(base, "logs")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	ctx := WithWorkingDir(context.Background(), base)
	resolved, err := ResolveLocalPath(ctx, filepath.Join("logs", "secret.txt"))
	if err != nil {
		t.Fatalf("expected symlink path to resolve, got error: %v", err)
	}
	expected, err := filepath.Abs(filepath.Join(base, "logs", "secret.txt"))
	if err != nil {
		t.Fatalf("failed to normalize expected symlink path: %v", err)
	}
	if resolved != expected {
		t.Fatalf("expected symlink path %q, got %q", expected, resolved)
	}
}

func TestResolveLocalPathAllowsMemoryRoot(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "path-guard-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		t.Skip("user home dir unavailable")
	}
	memoryRoot := filepath.Join(home, ".alex", "memory")

	ctx := WithWorkingDir(context.Background(), base)
	resolved, err := ResolveLocalPath(ctx, memoryRoot)
	if err != nil {
		t.Fatalf("expected memory root to resolve, got error: %v", err)
	}
	if resolved != filepath.Clean(memoryRoot) {
		t.Fatalf("expected resolved path %q, got %q", filepath.Clean(memoryRoot), resolved)
	}
}

func TestResolveLocalPathOrTemp_AllowsOsTempDirFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "path-guard-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})
	ctx := WithWorkingDir(context.Background(), base)

	tmpDir := os.TempDir()
	if strings.TrimSpace(tmpDir) == "" {
		t.Skip("os.TempDir is empty")
	}
	file, err := os.CreateTemp(tmpDir, "path-guard-*.txt")
	if err != nil {
		t.Skipf("failed to create temp file: %v", err)
	}
	path := file.Name()
	_ = file.Close()
	t.Cleanup(func() {
		_ = os.Remove(path)
	})

	resolved, err := ResolveLocalPathOrTemp(ctx, path)
	if err != nil {
		t.Fatalf("expected path to resolve, got error: %v", err)
	}
	expected, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		t.Fatalf("failed to normalize expected path: %v", err)
	}
	if resolved != expected {
		t.Fatalf("expected resolved path %q, got %q", expected, resolved)
	}
}

func TestResolveLocalPathOrTemp_AllowsNonTempOutsideBase(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "path-guard-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})
	ctx := WithWorkingDir(context.Background(), base)

	outside := filepath.Dir(base)
	resolved, err := ResolveLocalPathOrTemp(ctx, outside)
	if err != nil {
		t.Fatalf("expected outside path to resolve, got error: %v", err)
	}
	expected, err := filepath.Abs(filepath.Clean(outside))
	if err != nil {
		t.Fatalf("failed to normalize expected outside path: %v", err)
	}
	if resolved != expected {
		t.Fatalf("expected outside path %q, got %q", expected, resolved)
	}
}
