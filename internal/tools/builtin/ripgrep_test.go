package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"strings"
	"testing"
	"alex/internal/tools/builtin/shared"
)

func TestRipgrepMetadata(t *testing.T) {
	tool := NewRipgrep(shared.ShellToolConfig{})
	meta := tool.Metadata()

	if meta.Name != "ripgrep" {
		t.Errorf("Expected name 'ripgrep', got '%s'", meta.Name)
	}
	if meta.Category != "search" {
		t.Errorf("Expected category 'search', got '%s'", meta.Category)
	}
}

func TestRipgrepDefinition(t *testing.T) {
	tool := NewRipgrep(shared.ShellToolConfig{})
	def := tool.Definition()

	if def.Name != "ripgrep" {
		t.Errorf("Expected name 'ripgrep', got '%s'", def.Name)
	}
	if !strings.Contains(def.Description, "ripgrep") {
		t.Errorf("Description should mention ripgrep")
	}

	// Verify required parameters
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "pattern" {
		t.Errorf("Expected required parameter 'pattern', got %v", def.Parameters.Required)
	}

	// Verify parameter properties
	expectedProps := []string{"pattern", "path", "file_type", "ignore_case", "max_results"}
	for _, prop := range expectedProps {
		if _, ok := def.Parameters.Properties[prop]; !ok {
			t.Errorf("Missing parameter property: %s", prop)
		}
	}
}

func TestRipgrepMissingPattern(t *testing.T) {
	tool := NewRipgrep(shared.ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:        "test-1",
		Name:      "ripgrep",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Error == nil {
		t.Error("Expected error for missing pattern")
	}
	if !strings.Contains(result.Error.Error(), "pattern") {
		t.Errorf("Error should mention missing pattern, got: %v", result.Error)
	}
}

func TestRipgrepBasicSearch(t *testing.T) {
	tool := NewRipgrep(shared.ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:   "test-3",
		Name: "ripgrep",
		Arguments: map[string]any{
			"pattern": "package builtin",
			"path":    ".",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Error != nil {
		t.Errorf("Expected no error in result, got: %v", result.Error)
	}

	// Should find at least this file
	if !strings.Contains(result.Content, "ripgrep_test.go") && !strings.Contains(result.Content, "Found") {
		t.Errorf("Expected to find matches in result content")
	}
}

func TestRipgrepWithFileType(t *testing.T) {
	tool := NewRipgrep(shared.ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:   "test-4",
		Name: "ripgrep",
		Arguments: map[string]any{
			"pattern":   "package",
			"path":      ".",
			"file_type": "go",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Error != nil {
		t.Errorf("Expected no error in result, got: %v", result.Error)
	}

	// Should find matches in Go files
	if result.Metadata != nil {
		if matches, ok := result.Metadata["matches"].(int); ok && matches == 0 {
			t.Error("Expected to find matches in Go files")
		}
	}
}

func TestRipgrepNoMatches(t *testing.T) {
	tool := NewRipgrep(shared.ShellToolConfig{})
	ctx := context.Background()

	// Use a very unique string that won't be in the code
	uniquePattern := "XYZZY_UNIQUE_STRING_THAT_WILL_NEVER_EXIST_" + "IN_ANY_FILE_12345"

	call := ports.ToolCall{
		ID:   "test-5",
		Name: "ripgrep",
		Arguments: map[string]any{
			"pattern": uniquePattern,
			"path":    ".",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Error != nil {
		t.Errorf("Expected no error in result, got: %v", result.Error)
	}

	if !strings.Contains(result.Content, "No matches found") {
		t.Errorf("Expected 'No matches found' message, got: %s", result.Content)
	}

	if result.Metadata != nil {
		if matches, ok := result.Metadata["matches"].(int); ok && matches != 0 {
			t.Errorf("Expected 0 matches, got %d", matches)
		}
	}
}

func TestRipgrepIgnoreCase(t *testing.T) {
	tool := NewRipgrep(shared.ShellToolConfig{})
	ctx := context.Background()

	call := ports.ToolCall{
		ID:   "test-6",
		Name: "ripgrep",
		Arguments: map[string]any{
			"pattern":     "PACKAGE",
			"path":        ".",
			"ignore_case": true,
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Error != nil {
		t.Errorf("Expected no error in result, got: %v", result.Error)
	}

	// With ignore_case, should find lowercase "package"
	if result.Metadata != nil {
		if ignoreCase, ok := result.Metadata["ignore_case"].(bool); !ok || !ignoreCase {
			t.Error("Expected ignore_case metadata to be true")
		}
	}
}
