package subprocess

import (
	"context"
	"strings"
	"testing"
)

func TestSubprocess_StderrTailCapturesOutput(t *testing.T) {
	proc := New(Config{
		Command: "bash",
		Args:    []string{"-c", "echo err 1>&2; exit 2"},
	})
	if err := proc.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := proc.Wait(); err == nil {
		t.Fatalf("expected exit error")
	}

	if !strings.Contains(proc.StderrTail(), "err") {
		t.Fatalf("expected stderr tail to contain output, got %q", proc.StderrTail())
	}
}
