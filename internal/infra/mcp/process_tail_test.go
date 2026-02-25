package mcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProcessManager_StderrTailCapturesOutput(t *testing.T) {
	pm := NewProcessManager(ProcessConfig{
		Command: "bash",
		Args:    []string{"-c", "echo err 1>&2; exit 2"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pm.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	select {
	case <-pm.waitDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for process to exit")
	}

	tail := pm.StderrTail()
	if !strings.Contains(tail, "err") {
		t.Fatalf("expected stderr tail to contain output, got %q", tail)
	}
}
