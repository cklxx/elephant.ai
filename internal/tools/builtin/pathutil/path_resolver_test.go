package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewPathResolverNormalizesWorkingDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "path-resolver-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})
	dirty := filepath.Join(base, "child", "..")

	resolver := NewPathResolver(dirty)
	if resolver.workingDir != base {
		t.Fatalf("expected working dir %q, got %q", base, resolver.workingDir)
	}
}

func TestGetPathResolverFromContextNormalizesWorkingDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "path-resolver-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(base)
	})
	dirty := filepath.Join(base, "child", "..")

	ctx := WithWorkingDir(context.Background(), dirty)
	resolver := GetPathResolverFromContext(ctx)
	if resolver.workingDir != base {
		t.Fatalf("expected working dir %q, got %q", base, resolver.workingDir)
	}
}

func TestGetPathResolverFromContextFallsBackOnEmpty(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	expected, err := filepath.Abs(filepath.Clean(cwd))
	if err != nil {
		t.Fatalf("failed to normalize cwd: %v", err)
	}

	ctx := WithWorkingDir(context.Background(), "")
	resolver := GetPathResolverFromContext(ctx)
	if resolver.workingDir != expected {
		t.Fatalf("expected working dir %q, got %q", expected, resolver.workingDir)
	}
}
