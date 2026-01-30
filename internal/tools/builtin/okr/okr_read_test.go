package okr

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestOKRRead_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	tool := NewOKRRead(OKRConfig{GoalsRoot: dir})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Name:      "okr_read",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "No goals found") {
		t.Errorf("expected 'No goals found', got %q", result.Content)
	}
	count, ok := result.Metadata["count"]
	if !ok || count != 0 {
		t.Errorf("expected count=0, got %v", count)
	}
}

func TestOKRRead_ListWithGoals(t *testing.T) {
	dir := t.TempDir()
	cfg := OKRConfig{GoalsRoot: dir}
	store := NewGoalStore(cfg)

	if err := store.WriteGoalRaw("goal-a", []byte(sampleGoalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	tool := NewOKRRead(cfg)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-2",
		Name:      "okr_read",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "goal-a") {
		t.Errorf("expected 'goal-a' in output, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "active") {
		t.Errorf("expected 'active' status in output, got %q", result.Content)
	}
}

func TestOKRRead_SingleGoal(t *testing.T) {
	dir := t.TempDir()
	cfg := OKRConfig{GoalsRoot: dir}
	store := NewGoalStore(cfg)

	if err := store.WriteGoalRaw("my-goal", []byte(sampleGoalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	tool := NewOKRRead(cfg)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-3",
		Name: "okr_read",
		Arguments: map[string]any{
			"goal_id": "my-goal",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "q1-2026-revenue") {
		t.Errorf("expected goal content, got %q", result.Content)
	}
	if result.Metadata["goal_id"] != "my-goal" {
		t.Errorf("metadata goal_id = %v, want my-goal", result.Metadata["goal_id"])
	}
}

func TestOKRRead_GoalNotFound(t *testing.T) {
	dir := t.TempDir()
	tool := NewOKRRead(OKRConfig{GoalsRoot: dir})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-4",
		Name: "okr_read",
		Arguments: map[string]any{
			"goal_id": "nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for nonexistent goal")
	}
}

func TestOKRRead_Definition(t *testing.T) {
	tool := NewOKRRead(OKRConfig{GoalsRoot: t.TempDir()})
	def := tool.Definition()

	if def.Name != "okr_read" {
		t.Errorf("Name = %q, want okr_read", def.Name)
	}
	if def.Parameters.Properties["goal_id"].Type != "string" {
		t.Error("goal_id property should be string type")
	}
}

func TestOKRRead_Metadata(t *testing.T) {
	tool := NewOKRRead(OKRConfig{GoalsRoot: t.TempDir()})
	meta := tool.Metadata()

	if meta.Name != "okr_read" {
		t.Errorf("Name = %q, want okr_read", meta.Name)
	}
	if meta.Category != "okr" {
		t.Errorf("Category = %q, want okr", meta.Category)
	}
	if meta.Dangerous {
		t.Error("okr_read should not be dangerous")
	}
}
