package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindTool(t *testing.T) {
	find := NewFind(ShellToolConfig{})

	// Verify metadata
	meta := find.Metadata()
	if meta.Name != "find" {
		t.Errorf("expected name 'find', got '%s'", meta.Name)
	}

	// Verify definition
	def := find.Definition()
	if def.Name != "find" {
		t.Errorf("expected definition name 'find', got '%s'", def.Name)
	}
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "name" {
		t.Errorf("expected required parameter 'name', got %v", def.Parameters.Required)
	}
}

func TestFindExecute_GoFiles(t *testing.T) {
	find := NewFind(ShellToolConfig{})
	ctx := context.Background()

	// Test finding .go files in current directory
	call := ports.ToolCall{
		ID:   "test-1",
		Name: "find",
		Arguments: map[string]any{
			"name":      "*.go",
			"path":      ".",
			"max_depth": float64(2),
		},
	}

	result, err := find.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.CallID != "test-1" {
		t.Errorf("expected call_id 'test-1', got '%s'", result.CallID)
	}

	// Should find at least this test file
	if !strings.Contains(result.Content, ".go") {
		t.Errorf("expected to find .go files, got: %s", result.Content)
	}

	// Check metadata
	if result.Metadata["pattern"] != "*.go" {
		t.Errorf("expected pattern '*.go', got %v", result.Metadata["pattern"])
	}
}

func TestFindExecute_DirectoriesOnly(t *testing.T) {
	find := NewFind(ShellToolConfig{})
	ctx := context.Background()

	// Create a temporary test directory
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testsubdir")
	_ = os.Mkdir(testDir, 0755)

	call := ports.ToolCall{
		ID:   "test-2",
		Name: "find",
		Arguments: map[string]any{
			"name":      "test*",
			"path":      tmpDir,
			"type":      "d",
			"max_depth": float64(2),
		},
	}

	result, err := find.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}

	// Should find the testsubdir directory
	if !strings.Contains(result.Content, "testsubdir") {
		t.Errorf("expected to find testsubdir, got: %s", result.Content)
	}

	// Verify type filter was applied
	if result.Metadata["max_depth"] != 2 {
		t.Errorf("expected max_depth 2, got %v", result.Metadata["max_depth"])
	}
}

func TestFindExecute_NoMatches(t *testing.T) {
	find := NewFind(ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:   "test-3",
		Name: "find",
		Arguments: map[string]any{
			"name": "nonexistent-file-pattern-xyz-123.impossible",
			"path": ".",
		},
	}

	result, err := find.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != nil {
		t.Errorf("unexpected error for no matches: %v", result.Error)
	}

	if !strings.Contains(result.Content, "No matches found") {
		t.Errorf("expected 'No matches found', got: %s", result.Content)
	}

	// Check metadata
	if matches, ok := result.Metadata["matches"].(int); !ok || matches != 0 {
		t.Errorf("expected 0 matches, got %v", result.Metadata["matches"])
	}
}

func TestFindExecute_MissingName(t *testing.T) {
	find := NewFind(ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:        "test-4",
		Name:      "find",
		Arguments: map[string]any{},
	}

	result, err := find.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error == nil {
		t.Error("expected error for missing name parameter")
	}

	if !strings.Contains(result.Error.Error(), "name") {
		t.Errorf("expected error message about 'name', got: %v", result.Error)
	}
}

func TestFindExecute_DefaultPath(t *testing.T) {
	find := NewFind(ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:   "test-5",
		Name: "find",
		Arguments: map[string]any{
			"name": "*.go",
		},
	}

	result, err := find.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Default path should be "."
	if result.Metadata["path"] != "." {
		t.Errorf("expected default path '.', got %v", result.Metadata["path"])
	}
}

func TestFindExecute_DefaultMaxDepth(t *testing.T) {
	find := NewFind(ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:   "test-6",
		Name: "find",
		Arguments: map[string]any{
			"name": "*.go",
		},
	}

	result, err := find.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Default max_depth should be 10
	if result.Metadata["max_depth"] != 10 {
		t.Errorf("expected default max_depth 10, got %v", result.Metadata["max_depth"])
	}
}
