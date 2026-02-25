package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/domain/agent/ports"
	mem "alex/internal/infra/memory"
	id "alex/internal/shared/utils/id"
)

func TestMemoryRelatedRequiresUserID(t *testing.T) {
	engine := mem.NewMarkdownEngine(t.TempDir())
	tool := NewMemoryRelated(engine)

	call := ports.ToolCall{
		ID: "call-missing-user",
		Arguments: map[string]any{
			"path": "MEMORY.md",
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

func TestMemoryRelatedReturnsGraphHits(t *testing.T) {
	root := t.TempDir()
	engine := mem.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	dailyPath := filepath.Join(root, "memory", "2026-02-02.md")
	if err := os.MkdirAll(filepath.Dir(dailyPath), 0o755); err != nil {
		t.Fatalf("mkdir daily: %v", err)
	}
	if err := os.WriteFile(dailyPath, []byte("# 2026-02-02\n\n## Deploy\nDeployed v2.3.0 to production.\n"), 0o644); err != nil {
		t.Fatalf("write daily: %v", err)
	}

	longTerm := "# Long-Term Memory\n\nReview deploy details at [[memory:memory/2026-02-02.md#Deploy]].\n"
	if err := os.WriteFile(filepath.Join(root, "MEMORY.md"), []byte(longTerm), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}

	ctx := id.WithUserID(context.Background(), "user-1")
	tool := NewMemoryRelated(engine)
	call := ports.ToolCall{
		ID: "call-related",
		Arguments: map[string]any{
			"path":       "MEMORY.md",
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
	results, ok := result.Metadata["results"].([]mem.RelatedHit)
	if !ok || len(results) == 0 {
		t.Fatalf("expected related results in metadata, got %#v", result.Metadata["results"])
	}
	if results[0].Path != "memory/2026-02-02.md" {
		t.Fatalf("expected related path memory/2026-02-02.md, got %+v", results[0])
	}
}
