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

	if _, err := ResolveLocalPath(ctx, "../escape.txt"); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}

	outside := filepath.Dir(base)
	if _, err := ResolveLocalPath(ctx, outside); err == nil {
		t.Fatalf("expected absolute path outside base to be rejected")
	}
}

func TestResolveLocalPathRejectsSymlinkEscape(t *testing.T) {
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
	if _, err := ResolveLocalPath(ctx, filepath.Join("logs", "secret.txt")); err == nil {
		t.Fatalf("expected symlink escape to be rejected")
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

	if _, err := ResolveLocalPath(ctx, path); err == nil {
		t.Fatalf("expected ResolveLocalPath to reject path outside base")
	}
}

func TestResolveLocalPathOrTemp_RejectsNonTempOutsideBase(t *testing.T) {
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

	outside := string(filepath.Separator)
	if volume := filepath.VolumeName(base); volume != "" {
		outside = volume + string(filepath.Separator)
	}
	if tmp := filepath.Clean(os.TempDir()); tmp != "" && pathWithinBase(tmp, outside) {
		t.Skipf("cannot find outside-of-temp path (os.TempDir=%q)", tmp)
	}

	if _, err := ResolveLocalPathOrTemp(ctx, outside); err == nil {
		t.Fatalf("expected path outside base and temp to be rejected")
	}
}
