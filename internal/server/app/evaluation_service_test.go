package app

import (
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
