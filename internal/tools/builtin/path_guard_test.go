package builtin

import (
	"context"
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
