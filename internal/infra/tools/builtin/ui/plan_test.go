package ui

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	id "alex/internal/shared/utils/id"
)

func TestPlanExecuteSourcesRunIDFromContext(t *testing.T) {
	tool := NewPlan(nil)

	ctx := id.WithIDs(context.Background(), id.IDs{RunID: "ctx-run-123"})
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"overall_goal_ui": "Test goal",
			"complexity":      "simple",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	runID, ok := result.Metadata["run_id"].(string)
	if !ok || runID != "ctx-run-123" {
		t.Fatalf("expected run_id=ctx-run-123 from context, got %q", runID)
	}
}

func TestPlanExecuteIgnoresLLMSuppliedRunID(t *testing.T) {
	tool := NewPlan(nil)

	ctx := id.WithIDs(context.Background(), id.IDs{RunID: "ctx-run-456"})
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"run_id":          "llm-supplied-id",
			"overall_goal_ui": "Test goal",
			"complexity":      "simple",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	runID, ok := result.Metadata["run_id"].(string)
	if !ok || runID != "ctx-run-456" {
		t.Fatalf("expected run_id=ctx-run-456 from context (not LLM), got %q", runID)
	}
}

func TestPlanExecuteWithoutRunIDInContext(t *testing.T) {
	tool := NewPlan(nil)

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-3",
		Arguments: map[string]any{
			"overall_goal_ui": "Test goal",
			"complexity":      "simple",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	runID, ok := result.Metadata["run_id"].(string)
	if !ok || runID != "" {
		t.Fatalf("expected empty run_id when not in context, got %q", runID)
	}
}
