package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/memory"
	id "alex/internal/shared/utils/id"
)

func TestMemorySearchRequiresUserID(t *testing.T) {
	engine := memory.NewMarkdownEngine(t.TempDir())
	tool := NewMemorySearch(engine)

	call := ports.ToolCall{
		ID: "call-missing-user",
		Arguments: map[string]any{
			"query": "alpha",
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected error for missing user id")
	}
}

func TestMemorySearchReturnsResults(t *testing.T) {
	engine := memory.NewMarkdownEngine(t.TempDir())
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	when := time.Date(2026, 2, 2, 10, 0, 0, 0, time.Local)
	_, err := engine.AppendDaily(context.Background(), "user-1", memory.DailyEntry{
		Title:     "Deploy",
		Content:   "Deployed v2.3.0 to production.",
		CreatedAt: when,
	})
	if err != nil {
		t.Fatalf("AppendDaily: %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-1")
	tool := NewMemorySearch(engine)
	call := ports.ToolCall{
		ID: "call-search",
		Arguments: map[string]any{
			"query":      "production",
			"maxResults": 3,
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Metadata == nil {
		t.Fatalf("expected metadata to be populated")
	}
	results, ok := result.Metadata["results"].([]memory.SearchHit)
	if !ok || len(results) == 0 {
		t.Fatalf("expected results in metadata, got %#v", result.Metadata["results"])
	}
}

func TestMemoryGetReadsLines(t *testing.T) {
	root := t.TempDir()
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	userRoot := filepath.Join(root, "user-1")
	if err := os.MkdirAll(userRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(userRoot, "MEMORY.md")
	if err := os.WriteFile(path, []byte("# Memory\n\n- Alpha\n- Beta\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-1")
	tool := NewMemoryGet(engine)
	call := ports.ToolCall{
		ID: "call-get",
		Arguments: map[string]any{
			"path":  "MEMORY.md",
			"from":  2,
			"lines": 2,
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Content == "" {
		t.Fatalf("expected content")
	}
}
