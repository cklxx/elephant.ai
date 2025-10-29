package tools

import (
	"context"
	"testing"

	"alex/internal/diagnostics"
)

func TestSandboxManagerInitializePublishesErrorProgress(t *testing.T) {
	diagnostics.ResetSandboxProgressForTests()
	t.Cleanup(diagnostics.ResetSandboxProgressForTests)

	mgr := NewSandboxManager("")
	ctx := context.Background()
	err := mgr.Initialize(ctx)
	if err == nil {
		t.Fatalf("expected initialization error for empty base URL")
	}

	latest, ok := diagnostics.LatestSandboxProgress()
	if !ok {
		t.Fatalf("expected sandbox progress to be recorded")
	}
	if latest.Status != diagnostics.SandboxProgressError {
		t.Fatalf("expected error status, got %q", latest.Status)
	}
	if latest.Stage != "configure_client" {
		t.Fatalf("expected configure_client stage, got %q", latest.Stage)
	}
}
