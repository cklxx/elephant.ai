package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalPath(t *testing.T) {
	base := t.TempDir()
	ctx := WithWorkingDir(context.Background(), base)

	resolved, err := resolveLocalPath(ctx, "note.txt")
	if err != nil {
		t.Fatalf("expected path to resolve, got error: %v", err)
	}
	if !pathWithinBase(base, resolved) {
		t.Fatalf("expected resolved path %q to stay within base %q", resolved, base)
	}

	if _, err := resolveLocalPath(ctx, "../escape.txt"); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}

	outside := filepath.Dir(base)
	if _, err := resolveLocalPath(ctx, outside); err == nil {
		t.Fatalf("expected absolute path outside base to be rejected")
	}
}

func TestResolveLocalPathRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(base, "logs")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	ctx := WithWorkingDir(context.Background(), base)
	if _, err := resolveLocalPath(ctx, filepath.Join("logs", "secret.txt")); err == nil {
		t.Fatalf("expected symlink escape to be rejected")
	}
}
