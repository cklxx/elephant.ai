package config

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	runtimeconfig "alex/internal/shared/config"
)

func TestConfigManageGet(t *testing.T) {
	t.Parallel()
	tool := NewConfigManage()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "c1",
		Arguments: map[string]any{"action": "get"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Full config should contain some known key.
	if !strings.Contains(result.Content, "llm_provider") {
		t.Fatalf("expected llm_provider in output, got:\n%s", result.Content)
	}
}

func TestConfigManageGetKey(t *testing.T) {
	t.Parallel()
	tool := NewConfigManage()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "c2",
		Arguments: map[string]any{"action": "get", "key": "max_iterations"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.HasPrefix(result.Content, "max_iterations:") {
		t.Fatalf("expected max_iterations: prefix, got %q", result.Content)
	}
}

func TestConfigManageGetUnknownKey(t *testing.T) {
	t.Parallel()
	tool := NewConfigManage()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "c3",
		Arguments: map[string]any{"action": "get", "key": "nonexistent_key_xyz"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected error for unknown key, got content: %q", result.Content)
	}
}

func TestConfigManageSetSensitiveBlocked(t *testing.T) {
	t.Parallel()
	tool := NewConfigManage()
	for _, key := range []string{"api_key", "ark_api_key", "tavily_api_key", "moltbook_api_key", "app_secret"} {
		result, err := tool.Execute(context.Background(), ports.ToolCall{
			ID:        "c4",
			Arguments: map[string]any{"action": "set", "key": key, "value": "hack"},
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error == nil {
			t.Fatalf("expected error for sensitive key %q", key)
		}
		if !strings.Contains(result.Error.Error(), "sensitive") {
			t.Fatalf("expected sensitive error, got: %v", result.Error)
		}
	}
}

func TestConfigManageList(t *testing.T) {
	t.Parallel()
	tool := NewConfigManage()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "c5",
		Arguments: map[string]any{"action": "list"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "max_iterations") {
		t.Fatalf("expected max_iterations in list, got:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "[read-only]") {
		t.Fatalf("expected [read-only] for sensitive keys, got:\n%s", result.Content)
	}
}

func TestCoerceValue(t *testing.T) {
	t.Parallel()
	rc := runtimeconfig.RuntimeConfig{}

	tests := []struct {
		key    string
		value  string
		expect any
	}{
		{"llm_provider", "openai", "openai"},
		{"verbose", "true", true},
		{"verbose", "false", false},
		{"max_iterations", "42", 42},
		{"temperature", "0.5", 0.5},
	}
	for _, tt := range tests {
		got, err := coerceValue(rc, tt.key, tt.value)
		if err != nil {
			t.Fatalf("coerceValue(%q, %q): %v", tt.key, tt.value, err)
		}
		if got != tt.expect {
			t.Fatalf("coerceValue(%q, %q) = %v (%T), want %v (%T)", tt.key, tt.value, got, got, tt.expect, tt.expect)
		}
	}
}

func TestRuntimeFieldByYAMLTag(t *testing.T) {
	t.Parallel()
	rc := runtimeconfig.RuntimeConfig{MaxIterations: 99}
	val, ok := runtimeFieldByYAMLTag(rc, "max_iterations")
	if !ok {
		t.Fatalf("expected field found")
	}
	if val != 99 {
		t.Fatalf("expected 99, got %v", val)
	}

	_, ok = runtimeFieldByYAMLTag(rc, "nonexistent")
	if ok {
		t.Fatalf("expected field not found for nonexistent key")
	}
}
