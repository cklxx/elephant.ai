package builtin

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools"
)

func TestSandboxModeRequiresManager(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		tool ports.ToolExecutor
		call ports.ToolCall
	}{
		{
			name: "file_read",
			tool: NewFileRead(FileToolConfig{Mode: tools.ExecutionModeSandbox}),
			call: ports.ToolCall{ID: "1", Name: "file_read", Arguments: map[string]any{"path": "test.txt"}},
		},
		{
			name: "file_write",
			tool: NewFileWrite(FileToolConfig{Mode: tools.ExecutionModeSandbox}),
			call: ports.ToolCall{ID: "2", Name: "file_write", Arguments: map[string]any{"path": "test.txt", "content": "hello"}},
		},
		{
			name: "list_files",
			tool: NewListFiles(FileToolConfig{Mode: tools.ExecutionModeSandbox}),
			call: ports.ToolCall{ID: "3", Name: "list_files", Arguments: map[string]any{"path": "."}},
		},
		{
			name: "bash",
			tool: NewBash(ShellToolConfig{Mode: tools.ExecutionModeSandbox}),
			call: ports.ToolCall{ID: "4", Name: "bash", Arguments: map[string]any{"command": "echo hi"}},
		},
		{
			name: "grep",
			tool: NewGrep(ShellToolConfig{Mode: tools.ExecutionModeSandbox}),
			call: ports.ToolCall{ID: "5", Name: "grep", Arguments: map[string]any{"pattern": "foo", "path": "."}},
		},
		{
			name: "ripgrep",
			tool: NewRipgrep(ShellToolConfig{Mode: tools.ExecutionModeSandbox}),
			call: ports.ToolCall{ID: "6", Name: "ripgrep", Arguments: map[string]any{"pattern": "foo", "path": "."}},
		},
		{
			name: "find",
			tool: NewFind(ShellToolConfig{Mode: tools.ExecutionModeSandbox}),
			call: ports.ToolCall{ID: "7", Name: "find", Arguments: map[string]any{"path": ".", "name": "*.go"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.tool.Execute(ctx, tc.call)
			if err != nil {
				t.Fatalf("unexpected execute error: %v", err)
			}
			if result == nil {
				t.Fatalf("expected tool result")
			}
			if result.Error == nil {
				t.Fatalf("expected sandbox manager error")
			}
			if !strings.Contains(result.Error.Error(), "sandbox manager is required") {
				t.Fatalf("unexpected error message: %v", result.Error)
			}
		})
	}
}
