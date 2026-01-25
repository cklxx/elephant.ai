package builtin

import (
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type fileWrite struct {
}

func NewFileWrite(cfg FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &fileWrite{}
}

func (t *fileWrite) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'path'")}, nil
	}

	path = strings.TrimSpace(path)
	if path == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("file path cannot be empty")}, nil
	}

	content, ok := call.Arguments["content"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'content'")}, nil
	}

	resolved, err := resolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	return t.executeLocal(call, path, resolved, content), nil
}

func (t *fileWrite) executeLocal(call ports.ToolCall, path, resolved, content string) *ports.ToolResult {
	dir := filepath.Dir(resolved)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return &ports.ToolResult{CallID: call.ID, Error: err}
		}
	}

	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}
	}

	// Count lines in content
	lines := 0
	for _, ch := range content {
		if ch == '\n' {
			lines++
		}
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++ // Count last line if it doesn't end with newline
	}

	sum := sha256.Sum256([]byte(content))

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote %d bytes to %s", len(content), resolved),
		Metadata: map[string]any{
			"path":           path,
			"resolved_path":  resolved,
			"chars":          len(content),
			"lines":          lines,
			"content_len":    len(content),
			"content_sha256": fmt.Sprintf("%x", sum),
		},
	}
}

func (t *fileWrite) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "file_write",
		Description: "Write content to file",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path":    {Type: "string", Description: "File path"},
				"content": {Type: "string", Description: "Content to write"},
			},
			Required: []string{"path", "content"},
		},
	}
}

func (t *fileWrite) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "file_write", Version: "1.0.0", Category: "file_operations", Dangerous: true,
	}
}
