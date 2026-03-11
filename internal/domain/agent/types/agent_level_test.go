package types

import (
	"context"
	"testing"

	agentports "alex/internal/domain/agent/ports/agent"
)

func TestWithOutputContextStoresAgentOutputContext(t *testing.T) {
	outCtx := &OutputContext{
		Level:        LevelParallel,
		Category:     CategoryExecution,
		AgentID:      "worker-1",
		ParentID:     "root",
		Verbose:      true,
		SessionID:    "session-1",
		TaskID:       "task-1",
		ParentTaskID: "task-0",
		LogID:        "log-1",
	}

	ctx := WithOutputContext(context.Background(), outCtx)
	got := agentports.GetOutputContext(ctx)
	if got != outCtx {
		t.Fatalf("expected stored output context pointer to round-trip, got %+v", got)
	}
}
