package workdir

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/infra/tools/builtin/pathutil"
)

func TestDefaultWorkingDir_ReturnsNonEmpty(t *testing.T) {
	dir := DefaultWorkingDir()
	if dir == "" {
		t.Fatal("DefaultWorkingDir returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("DefaultWorkingDir returned non-absolute path: %s", dir)
	}
}

func TestDefaultWorkingDir_MatchesOsGetwd(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	expected, err := filepath.Abs(filepath.Clean(cwd))
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	got := DefaultWorkingDir()
	if got != expected {
		t.Fatalf("DefaultWorkingDir = %q, want %q", got, expected)
	}
}

func TestWithWorkingDir_StoresValueInContext(t *testing.T) {
	ctx := context.Background()
	dir := "/tmp/test-workdir"
	ctx = WithWorkingDir(ctx, dir)

	got, ok := ctx.Value(pathutil.WorkingDirKey).(string)
	if !ok {
		t.Fatal("WithWorkingDir did not store value with expected key")
	}
	if got != dir {
		t.Fatalf("stored value = %q, want %q", got, dir)
	}
}

func TestWithWorkingDir_EmptyString(t *testing.T) {
	ctx := WithWorkingDir(context.Background(), "")
	got, ok := ctx.Value(pathutil.WorkingDirKey).(string)
	if !ok {
		t.Fatal("WithWorkingDir did not store value for empty string")
	}
	if got != "" {
		t.Fatalf("stored value = %q, want empty string", got)
	}
}

func TestWithWorkingDir_OverridesPreviousValue(t *testing.T) {
	ctx := WithWorkingDir(context.Background(), "/first")
	ctx = WithWorkingDir(ctx, "/second")

	got, ok := ctx.Value(pathutil.WorkingDirKey).(string)
	if !ok {
		t.Fatal("WithWorkingDir did not store value")
	}
	if got != "/second" {
		t.Fatalf("stored value = %q, want /second", got)
	}
}

func TestWithWorkingDir_PreservesOtherContextValues(t *testing.T) {
	type otherKey string
	ctx := context.WithValue(context.Background(), otherKey("foo"), "bar")
	ctx = WithWorkingDir(ctx, "/mydir")

	if v := ctx.Value(otherKey("foo")); v != "bar" {
		t.Fatalf("other context value lost: got %v, want bar", v)
	}
	got, ok := ctx.Value(pathutil.WorkingDirKey).(string)
	if !ok || got != "/mydir" {
		t.Fatalf("working dir value = %q, want /mydir", got)
	}
}

func TestWithWorkingDir_NilContextPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic with nil context, got none")
		}
	}()
	// context.WithValue panics on nil context.
	WithWorkingDir(nil, "/test") //nolint:staticcheck
}

func TestWithWorkingDir_RoundtripThroughPathResolver(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	base, err := os.MkdirTemp(cwd, "workdir-test-")
	if err != nil {
		t.Fatalf("os.MkdirTemp failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })

	ctx := WithWorkingDir(context.Background(), base)
	resolver := pathutil.GetPathResolverFromContext(ctx)
	resolved := resolver.ResolvePath(".")
	if resolved != base {
		t.Fatalf("resolver.ResolvePath(\".\") = %q, want %q", resolved, base)
	}
}
