package toolregistry

import (
	"context"
	"testing"

	"alex/internal/infra/memory"
)

func TestRegistry_FindToolRegistered(t *testing.T) {
	engine := memory.NewMarkdownEngine(t.TempDir())
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	registry, err := NewRegistry(Config{MemoryEngine: engine})
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	// Test that find tool is registered
	tool, err := registry.Get("find")
	if err != nil {
		t.Fatalf("find tool not registered: %v", err)
	}

	// Verify metadata
	meta := tool.Metadata()
	if meta.Name != "find" {
		t.Errorf("expected tool name 'find', got '%s'", meta.Name)
	}

	// Verify definition
	def := tool.Definition()
	if def.Name != "find" {
		t.Errorf("expected definition name 'find', got '%s'", def.Name)
	}

	// Verify it's in the list
	defs := registry.List()
	foundInList := false
	for _, d := range defs {
		if d.Name == "find" {
			foundInList = true
			break
		}
	}
	if !foundInList {
		t.Error("find tool not found in registry.List()")
	}
}
