package builtin

import (
	"testing"
)

func TestWebFetchTool_Basic(t *testing.T) {
	tool := CreateWebFetchTool()

	// Test tool metadata
	if tool.Name() != "web_fetch" {
		t.Errorf("Expected name 'web_fetch', got %s", tool.Name())
	}

	description := tool.Description()
	if description == "" {
		t.Error("Description should not be empty")
	}

	params := tool.Parameters()
	if params == nil {
		t.Error("Parameters should not be nil")
	}
}

func TestWebFetchTool_Validation(t *testing.T) {
	tool := CreateWebFetchTool()

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{
			name: "valid args",
			args: map[string]any{
				"url":    "https://example.com",
				"prompt": "What is this page about?",
			},
			wantErr: false,
		},
		{
			name: "missing url",
			args: map[string]any{
				"prompt": "What is this page about?",
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			args: map[string]any{
				"url": "https://example.com",
			},
			wantErr: true,
		},
		{
			name: "invalid url",
			args: map[string]any{
				"url":    "not-a-url",
				"prompt": "What is this page about?",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWebFetchTool_CacheKey(t *testing.T) {
	tool := CreateWebFetchTool()

	key1 := tool.getCacheKey("https://example.com")
	key2 := tool.getCacheKey("https://example.com")
	key3 := tool.getCacheKey("https://different.com")

	if key1 != key2 {
		t.Error("Same URLs should generate same cache keys")
	}

	if key1 == key3 {
		t.Error("Different URLs should generate different cache keys")
	}
}

func TestWebFetchTool_GetHost(t *testing.T) {
	tool := CreateWebFetchTool()

	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com", "example.com"},
		{"https://example.com/path", "example.com"},
		{"http://sub.example.com", "sub.example.com"},
		{"invalid-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			host := tool.getHost(tt.url)
			if host != tt.expected {
				t.Errorf("getHost(%s) = %s, want %s", tt.url, host, tt.expected)
			}
		})
	}
}

// Note: We skip testing Execute() method as it requires real HTTP requests and LLM calls
// This would be better suited for integration tests