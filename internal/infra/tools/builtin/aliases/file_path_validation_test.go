package aliases

import (
	"context"
	"os"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

func TestFileToolsRejectPathTraversal(t *testing.T) {
	baseDir := pathutil.DefaultWorkingDir()
	if baseDir == "" {
		t.Fatalf("default working dir is empty")
	}
	tempDir, err := os.MkdirTemp(baseDir, "file-path-validation-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)

	tests := []struct {
		name string
		tool any
		call ports.ToolCall
	}{
		{
			name: "read_file",
			tool: NewReadFile(shared.FileToolConfig{}),
			call: ports.ToolCall{
				ID:   "call-read",
				Name: "read_file",
				Arguments: map[string]any{
					"path": "../../etc/passwd",
				},
			},
		},
		{
			name: "write_file",
			tool: NewWriteFile(shared.FileToolConfig{}),
			call: ports.ToolCall{
				ID:   "call-write",
				Name: "write_file",
				Arguments: map[string]any{
					"path":    "../../etc/passwd",
					"content": "blocked",
				},
			},
		},
		{
			name: "replace_in_file",
			tool: NewReplaceInFile(shared.FileToolConfig{}),
			call: ports.ToolCall{
				ID:   "call-replace",
				Name: "replace_in_file",
				Arguments: map[string]any{
					"path":    "../../etc/passwd",
					"old_str": "root",
					"new_str": "blocked",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, ok := tt.tool.(interface {
				Execute(context.Context, ports.ToolCall) (*ports.ToolResult, error)
			})
			if !ok {
				t.Fatalf("tool does not implement Execute")
			}
			result, err := executor.Execute(ctx, tt.call)
			if err != nil {
				t.Fatalf("unexpected Execute error: %v", err)
			}
			if result == nil || result.Error == nil {
				t.Fatalf("expected tool result error for traversal path")
			}
			if !strings.Contains(result.Error.Error(), "escapes workspace root") {
				t.Fatalf("expected workspace escape error, got %v", result.Error)
			}
		})
	}
}
