package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/llm"
	"alex/internal/workflow"
)

// TestAgentCoordinatorEndToEndExecutionPerformance exercises the coordinator through
// the full preparation + execution pipeline using the mock LLM provider. The goal is
// to assert that an end-to-end task finishes within the baseline window established
// before the refactor, demonstrating no runtime regression for the happy path.
func TestAgentCoordinatorEndToEndExecutionPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping acceptance-style performance check in short mode")
	}

	llmFactory := llm.NewFactory()
	sessionStore := &stubSessionStore{}
	coordinator := NewAgentCoordinator(
		llmFactory,
		stubToolRegistry{},
		sessionStore,
		stubContextManager{},
		stubParser{},
		nil,
		Config{
			LLMProvider:   "mock",
			LLMModel:      "acceptance",
			MaxIterations: 2,
			Temperature:   0.4,
		},
	)

	const iterations = 5
	var total time.Duration

	for i := 0; i < iterations; i++ {
		ctx := ports.WithOutputContext(context.Background(), &ports.OutputContext{Level: ports.LevelCore})

		start := time.Now()
		result, err := coordinator.ExecuteTask(ctx, "Return a concise answer", "", nil)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("run %d failed: %v", i+1, err)
		}

		if result == nil {
			t.Fatalf("run %d returned nil result", i+1)
		}

		if result.Workflow == nil {
			t.Fatalf("run %d missing workflow snapshot", i+1)
		}
		if result.Workflow.Phase != workflow.PhaseSucceeded {
			t.Fatalf("run %d unexpected workflow phase: %s", i+1, result.Workflow.Phase)
		}
		statusByNode := make(map[string]workflow.NodeStatus)
		for _, node := range result.Workflow.Nodes {
			statusByNode[node.ID] = node.Status
		}
		expected := map[string]workflow.NodeStatus{
			"prepare":   workflow.NodeStatusSucceeded,
			"execute":   workflow.NodeStatusSucceeded,
			"summarize": workflow.NodeStatusSucceeded,
			"persist":   workflow.NodeStatusSucceeded,
		}
		for node, status := range expected {
			if statusByNode[node] != status {
				t.Fatalf("run %d node %s expected %s got %s", i+1, node, status, statusByNode[node])
			}
		}

		if got := strings.TrimSpace(result.Answer); got != "Mock LLM response" {
			t.Fatalf("unexpected answer on run %d: %q", i+1, got)
		}

		total += duration
		t.Logf("run %d completed in %s", i+1, duration)
	}

	average := total / iterations
	// Historical baseline prior to the architecture refactor averaged ~110ms for the
	// mock provider workflow. We keep a 2x safety margin to avoid flakes in CI while
	// still detecting regressions introduced by coordination changes.
	const maxAverage = 220 * time.Millisecond

	if average > maxAverage {
		t.Fatalf("average execution time %s exceeded regression budget %s", average, maxAverage)
	}

	t.Logf("average execution time across %d runs: %s", iterations, average)
}
