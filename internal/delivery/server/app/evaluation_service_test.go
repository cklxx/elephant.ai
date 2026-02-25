package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSafeBaseDirRejectsTraversal(t *testing.T) {
	if err := ensureSafeBaseDir("../escape"); err == nil {
		t.Fatalf("expected traversal base path to be rejected")
	}
}

func TestEnsureOutputDirWithinBase(t *testing.T) {
	base := t.TempDir()
	svc := &EvaluationService{baseOutputDir: base}

	inside := filepath.Join(base, "job-123")
	if err := svc.ensureOutputDir(inside); err != nil {
		t.Fatalf("expected inside path to be accepted: %v", err)
	}

	out := filepath.Join(base, "..", "evil")
	if err := svc.ensureOutputDir(out); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}
}

func TestSafeOutputDirHandlesBaseSelfAndAbsolutePaths(t *testing.T) {
	base := t.TempDir()
	baseCanonical := canonicalizePath(base)
	svc := &EvaluationService{baseOutputDir: base}

	if got := svc.safeOutputDir(base); got != baseCanonical {
		t.Fatalf("expected base to normalize to itself, got %q", got)
	}
	if got := svc.safeOutputDir(filepath.Base(base)); got != baseCanonical {
		t.Fatalf("expected base name to map to base, got %q", got)
	}
	job := filepath.Join(baseCanonical, "job-1")
	if got := svc.safeOutputDir(job); got != job {
		t.Fatalf("expected absolute inside base to be accepted, got %q", got)
	}
	if got := svc.safeOutputDir(filepath.Join(base, "..", "evil")); got != baseCanonical {
		t.Fatalf("expected absolute outside base to clamp to base, got %q", got)
	}
}

func TestEnsureOutputDirMixedAbsoluteRelativeDoesNotError(t *testing.T) {
	temp := t.TempDir()
	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// base is relative, output is absolute within base -> should not error.
	baseRel := "evaluation_results"
	baseAbs := filepath.Join(temp, baseRel)
	if err := os.MkdirAll(baseAbs, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	svc := &EvaluationService{baseOutputDir: baseRel}
	outputAbs := filepath.Join(canonicalizePath(baseAbs), "job-123")
	if err := svc.ensureOutputDir(outputAbs); err != nil {
		t.Fatalf("expected abs output within rel base to be accepted, got %v", err)
	}
}
