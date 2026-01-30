package okr

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
)

func TestOKRWrite_Create(t *testing.T) {
	dir := t.TempDir()
	cfg := OKRConfig{GoalsRoot: dir}
	tool := NewOKRWrite(cfg)

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "okr_write",
		Arguments: map[string]any{
			"goal_id": "new-goal",
			"content": sampleGoalContent,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "saved") {
		t.Errorf("expected 'saved' in result, got %q", result.Content)
	}

	// Verify it was written
	store := NewGoalStore(cfg)
	goal, err := store.ReadGoal("new-goal")
	if err != nil {
		t.Fatalf("ReadGoal: %v", err)
	}
	if goal.Meta.ID != "q1-2026-revenue" {
		t.Errorf("ID = %q, want q1-2026-revenue", goal.Meta.ID)
	}
	// Verify updated date was auto-set to today
	today := time.Now().Format("2006-01-02")
	if goal.Meta.Updated != today {
		t.Errorf("Updated = %q, want %q", goal.Meta.Updated, today)
	}
}

func TestOKRWrite_MissingGoalID(t *testing.T) {
	tool := NewOKRWrite(OKRConfig{GoalsRoot: t.TempDir()})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-2",
		Name: "okr_write",
		Arguments: map[string]any{
			"content": sampleGoalContent,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing goal_id")
	}
}

func TestOKRWrite_MissingContent(t *testing.T) {
	tool := NewOKRWrite(OKRConfig{GoalsRoot: t.TempDir()})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-3",
		Name: "okr_write",
		Arguments: map[string]any{
			"goal_id": "test",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestOKRWrite_InvalidContent(t *testing.T) {
	tool := NewOKRWrite(OKRConfig{GoalsRoot: t.TempDir()})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-4",
		Name: "okr_write",
		Arguments: map[string]any{
			"goal_id": "bad",
			"content": "not a valid goal file",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid content")
	}
}

func TestOKRWrite_MetadataReturned(t *testing.T) {
	tool := NewOKRWrite(OKRConfig{GoalsRoot: t.TempDir()})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-5",
		Name: "okr_write",
		Arguments: map[string]any{
			"goal_id": "meta-test",
			"content": sampleGoalContent,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	if result.Metadata["goal_id"] != "meta-test" {
		t.Errorf("metadata goal_id = %v, want meta-test", result.Metadata["goal_id"])
	}
	if result.Metadata["status"] != "active" {
		t.Errorf("metadata status = %v, want active", result.Metadata["status"])
	}
	if result.Metadata["kr_count"] != 2 {
		t.Errorf("metadata kr_count = %v, want 2", result.Metadata["kr_count"])
	}
}

func TestOKRWrite_Definition(t *testing.T) {
	tool := NewOKRWrite(OKRConfig{GoalsRoot: t.TempDir()})
	def := tool.Definition()

	if def.Name != "okr_write" {
		t.Errorf("Name = %q, want okr_write", def.Name)
	}
	if len(def.Parameters.Required) != 2 {
		t.Errorf("Required = %v, want [goal_id, content]", def.Parameters.Required)
	}
}

func TestOKRWrite_Metadata(t *testing.T) {
	tool := NewOKRWrite(OKRConfig{GoalsRoot: t.TempDir()})
	meta := tool.Metadata()

	if meta.Name != "okr_write" {
		t.Errorf("Name = %q, want okr_write", meta.Name)
	}
	if meta.Category != "okr" {
		t.Errorf("Category = %q, want okr", meta.Category)
	}
	if meta.Dangerous {
		t.Error("okr_write should not be dangerous")
	}
}
