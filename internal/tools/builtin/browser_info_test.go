package builtin

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools"
)

func TestBrowserInfoEnforcesSandboxMode(t *testing.T) {
	tool := NewBrowserInfo(BrowserToolConfig{Mode: tools.ExecutionModeLocal})
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "1"})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected sandbox enforcement error")
	}
}

func TestBrowserInfoRequiresManager(t *testing.T) {
	tool := NewBrowserInfo(BrowserToolConfig{Mode: tools.ExecutionModeSandbox})
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "2"})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected sandbox manager requirement error")
	}
}
