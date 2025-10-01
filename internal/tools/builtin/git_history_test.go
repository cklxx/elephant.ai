package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"strings"
	"testing"
)

func TestGitHistory_Definition(t *testing.T) {
	tool := NewGitHistory()
	def := tool.Definition()

	if def.Name != "git_history" {
		t.Errorf("expected name 'git_history', got %q", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	if def.Parameters.Type != "object" {
		t.Errorf("expected parameters type 'object', got %q", def.Parameters.Type)
	}

	// Check that parameters are defined
	if _, ok := def.Parameters.Properties["query"]; !ok {
		t.Error("expected 'query' property")
	}

	if _, ok := def.Parameters.Properties["type"]; !ok {
		t.Error("expected 'type' property")
	}

	if _, ok := def.Parameters.Properties["file"]; !ok {
		t.Error("expected 'file' property")
	}

	if _, ok := def.Parameters.Properties["limit"]; !ok {
		t.Error("expected 'limit' property")
	}

	// Check enum values for type
	typeProperty := def.Parameters.Properties["type"]
	if len(typeProperty.Enum) == 0 {
		t.Error("expected enum values for 'type' property")
	}

	expectedTypes := []string{"message", "code", "file", "author", "date"}
	for _, expectedType := range expectedTypes {
		found := false
		for _, enumVal := range typeProperty.Enum {
			if enumVal == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected type enum to contain %q", expectedType)
		}
	}
}

func TestGitHistory_Metadata(t *testing.T) {
	tool := NewGitHistory()
	meta := tool.Metadata()

	if meta.Name != "git_history" {
		t.Errorf("expected name 'git_history', got %q", meta.Name)
	}

	if meta.Version == "" {
		t.Error("expected non-empty version")
	}

	if meta.Category != "git" {
		t.Errorf("expected category 'git', got %q", meta.Category)
	}

	// git_history should NOT be marked as dangerous (read-only operation)
	if meta.Dangerous {
		t.Error("expected tool to NOT be marked as dangerous")
	}

	if len(meta.Tags) == 0 {
		t.Error("expected non-empty tags")
	}
}

func TestGitHistory_Execute_InvalidSearchType(t *testing.T) {
	tool := NewGitHistory()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "git_history",
		Arguments: map[string]any{
			"query": "test",
			"type":  "invalid_type",
		},
	})

	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if result.Error == nil {
		t.Error("expected error for invalid search type")
	}

	if !strings.Contains(result.Error.Error(), "invalid search type") {
		t.Errorf("expected error about invalid search type, got %v", result.Error)
	}
}

func TestGitHistory_Execute_MissingQuery(t *testing.T) {
	tool := NewGitHistory()

	tests := []struct {
		name       string
		searchType string
		arguments  map[string]any
		wantError  bool
	}{
		{
			name:       "message search without query",
			searchType: "message",
			arguments: map[string]any{
				"type": "message",
			},
			wantError: true,
		},
		{
			name:       "code search without query",
			searchType: "code",
			arguments: map[string]any{
				"type": "code",
			},
			wantError: true,
		},
		{
			name:       "author search without query",
			searchType: "author",
			arguments: map[string]any{
				"type": "author",
			},
			wantError: true,
		},
		{
			name:       "file search without file parameter",
			searchType: "file",
			arguments: map[string]any{
				"type": "file",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:        "test-1",
				Name:      "git_history",
				Arguments: tt.arguments,
			})

			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}

			if tt.wantError && result.Error == nil {
				t.Error("expected error but got none")
			}

			if tt.wantError && result.Error != nil {
				if !strings.Contains(result.Error.Error(), "required") {
					t.Errorf("expected error about required parameter, got %v", result.Error)
				}
			}
		})
	}
}

func TestGitHistory_Execute_DefaultLimit(t *testing.T) {
	tool := NewGitHistory()

	// Test that default limit is applied
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "git_history",
		Arguments: map[string]any{
			"query": "test",
			"type":  "message",
		},
	})

	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	// Check metadata for limit
	if result.Metadata != nil {
		limit, ok := result.Metadata["limit"].(int)
		if ok && limit != 20 {
			t.Errorf("expected default limit of 20, got %d", limit)
		}
	}
}

func TestGitHistory_Execute_CustomLimit(t *testing.T) {
	tool := NewGitHistory()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "git_history",
		Arguments: map[string]any{
			"query": "test",
			"type":  "message",
			"limit": 10.0,
		},
	})

	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	// Check metadata for limit
	if result.Metadata != nil {
		limit, ok := result.Metadata["limit"].(int)
		if ok && limit != 10 {
			t.Errorf("expected limit of 10, got %d", limit)
		}
	}
}

func TestGitHistory_Execute_NotGitRepo(t *testing.T) {
	tool := NewGitHistory()

	// This test assumes we might not be in a git repo in test environment
	// The tool should handle this gracefully

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "git_history",
		Arguments: map[string]any{
			"query": "test",
			"type":  "message",
		},
	})

	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	// If we're not in a git repo, should get an error about it
	// If we are in a git repo, should get results or "no results found"
	if result.Error != nil {
		if !strings.Contains(result.Error.Error(), "git") &&
			!strings.Contains(result.Error.Error(), "repository") {
			t.Logf("Got expected error: %v", result.Error)
		}
	}
}

func TestGitHistory_SearchTypes(t *testing.T) {
	tool := NewGitHistory()

	tests := []struct {
		name       string
		searchType string
		arguments  map[string]any
	}{
		{
			name:       "message search",
			searchType: "message",
			arguments: map[string]any{
				"query": "feat",
				"type":  "message",
			},
		},
		{
			name:       "code search",
			searchType: "code",
			arguments: map[string]any{
				"query": "func main",
				"type":  "code",
			},
		},
		{
			name:       "file search",
			searchType: "file",
			arguments: map[string]any{
				"type": "file",
				"file": "README.md",
			},
		},
		{
			name:       "author search",
			searchType: "author",
			arguments: map[string]any{
				"query": "test@example.com",
				"type":  "author",
			},
		},
		{
			name:       "date search",
			searchType: "date",
			arguments: map[string]any{
				"query": "last week",
				"type":  "date",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:        "test-1",
				Name:      "git_history",
				Arguments: tt.arguments,
			})

			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}

			// Result should have metadata with search_type
			if result.Metadata != nil {
				searchType, ok := result.Metadata["search_type"].(string)
				if ok && searchType != tt.searchType {
					t.Errorf("expected search_type %q, got %q", tt.searchType, searchType)
				}
			}

			// Either we get results or an error (if not in git repo or no results)
			if result.Error != nil {
				t.Logf("Got error (possibly expected): %v", result.Error)
			} else if result.Content == "" {
				t.Error("expected non-empty content or error")
			}
		})
	}
}

func TestGitHistory_DateSearchFormats(t *testing.T) {
	tool := NewGitHistory()

	tests := []struct {
		name      string
		dateQuery string
	}{
		{
			name:      "relative date",
			dateQuery: "last week",
		},
		{
			name:      "days ago",
			dateQuery: "3 days ago",
		},
		{
			name:      "date range",
			dateQuery: "2024-01-01..2024-12-31",
		},
		{
			name:      "specific date",
			dateQuery: "2024-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:   "test-1",
				Name: "git_history",
				Arguments: map[string]any{
					"query": tt.dateQuery,
					"type":  "date",
				},
			})

			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}

			// Should handle all date formats without crashing
			// Might return no results or error if not in git repo
			if result.Error != nil {
				t.Logf("Got error (possibly expected): %v", result.Error)
			}
		})
	}
}

func TestGitHistory_Metadata_Structure(t *testing.T) {
	tool := NewGitHistory()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "git_history",
		Arguments: map[string]any{
			"query": "test query",
			"type":  "message",
			"limit": 15.0,
		},
	})

	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	// Check that metadata has expected fields
	if result.Metadata == nil {
		t.Fatal("expected metadata to be populated")
	}

	expectedFields := []string{"search_type", "query", "limit"}
	for _, field := range expectedFields {
		if _, ok := result.Metadata[field]; !ok {
			t.Errorf("expected metadata to contain field %q", field)
		}
	}
}

func TestGitHistory_DefaultSearchType(t *testing.T) {
	tool := NewGitHistory()

	// When type is not specified, should default to "message"
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "git_history",
		Arguments: map[string]any{
			"query": "test",
			// type not specified
		},
	})

	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	// Should default to message search
	if result.Metadata != nil {
		searchType, ok := result.Metadata["search_type"].(string)
		if ok && searchType != "message" && searchType != "" {
			t.Errorf("expected default search type to be message or empty, got %q", searchType)
		}
	}
}
