package ui

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestRequestUserExecuteWithOptions(t *testing.T) {
	tool := NewRequestUser()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"message": "请选择部署环境",
			"title":   "需要你的选择",
			"options": []any{"dev", "staging", "prod"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success result, got %#v", result)
	}
	if needs, ok := result.Metadata["needs_user_input"].(bool); !ok || !needs {
		t.Fatalf("expected needs_user_input=true in metadata")
	}
	options, ok := result.Metadata["options"].([]string)
	if !ok || len(options) != 3 || options[0] != "dev" || options[1] != "staging" || options[2] != "prod" {
		t.Fatalf("expected options in metadata, got %#v", result.Metadata["options"])
	}
	if !strings.Contains(result.Content, "需要你的选择") {
		t.Fatalf("expected title in content, got %q", result.Content)
	}
}

func TestRequestUserExecuteRejectsInvalidOptions(t *testing.T) {
	tool := NewRequestUser()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"message": "请选择部署环境",
			"options": "dev",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected tool error result, got %#v", result)
	}
	if !strings.Contains(result.Error.Error(), "options must be an array") {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}
