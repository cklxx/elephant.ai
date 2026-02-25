package ui

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestContextCheckpointMissingSummary(t *testing.T) {
	tool := NewContextCheckpoint()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing summary")
	}
}

func TestContextCheckpointEmptySummary(t *testing.T) {
	tool := NewContextCheckpoint()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{"summary": "   "},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for empty summary")
	}
}

func TestContextCheckpointSummaryTooShort(t *testing.T) {
	tool := NewContextCheckpoint()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{"summary": "too short"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for summary < 50 chars")
	}
	if !strings.Contains(result.Content, "50 characters") {
		t.Fatalf("expected error to mention 50 characters, got: %s", result.Content)
	}
}

func TestContextCheckpointValidParameters(t *testing.T) {
	tool := NewContextCheckpoint()

	summary := strings.Repeat("x", 60)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"summary":     summary,
			"phase_label": "research",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if result.Metadata["phase"] != "research" {
		t.Fatalf("expected phase=research, got %v", result.Metadata["phase"])
	}
	if result.Metadata["action"] != "checkpoint" {
		t.Fatalf("expected action=checkpoint, got %v", result.Metadata["action"])
	}
}

func TestContextCheckpointDefaultPhaseLabel(t *testing.T) {
	tool := NewContextCheckpoint()

	summary := strings.Repeat("x", 60)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"summary": summary,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if result.Metadata["phase"] != "phase" {
		t.Fatalf("expected default phase label 'phase', got %v", result.Metadata["phase"])
	}
}

func TestContextCheckpointUnsupportedParameter(t *testing.T) {
	tool := NewContextCheckpoint()

	summary := strings.Repeat("x", 60)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"summary": summary,
			"unknown": "value",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unsupported parameter")
	}
}
