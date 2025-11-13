package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	api "github.com/agent-infra/sandbox-sdk-go"
)

type fileWrite struct {
	mode    tools.ExecutionMode
	sandbox *tools.SandboxManager
}

func NewFileWrite(cfg FileToolConfig) ports.ToolExecutor {
	mode := cfg.Mode
	if mode == tools.ExecutionModeUnknown {
		mode = tools.ExecutionModeLocal
	}
	return &fileWrite{mode: mode, sandbox: cfg.SandboxManager}
}

func (t *fileWrite) Mode() tools.ExecutionMode {
	return t.mode
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

	if t.mode == tools.ExecutionModeSandbox {
		return t.executeSandbox(ctx, call, path, content)
	}
	return t.executeLocal(call, path, content), nil
}

func (t *fileWrite) executeLocal(call ports.ToolCall, path, content string) *ports.ToolResult {
	resolved := filepath.Clean(path)

	if err := ensureParentDirectory(resolved); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}
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

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote %d bytes to %s", len(content), resolved),
		Metadata: map[string]any{
			"path":    resolved,
			"chars":   len(content),
			"lines":   lines,
			"content": content,
		},
	}
}

func (t *fileWrite) executeSandbox(ctx context.Context, call ports.ToolCall, path, content string) (*ports.ToolResult, error) {
	if t.sandbox == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("sandbox manager is required")}, nil
	}
	if err := t.sandbox.Initialize(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}

	resolvedPath, err := resolveSandboxPath(path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	req := &api.FileWriteRequest{File: resolvedPath, Content: content}
	resp, err := t.sandbox.File().WriteFile(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}
	bytesWritten := len(content)
	if data := resp.GetData(); data != nil && data.GetBytesWritten() != nil {
		bytesWritten = *data.GetBytesWritten()
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

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote %d bytes to %s", bytesWritten, resolvedPath),
		Metadata: map[string]any{
			"path":    resolvedPath,
			"chars":   bytesWritten,
			"lines":   lines,
			"content": content,
		},
	}, nil
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

func ensureParentDirectory(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func resolveSandboxPath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		if cleaned == "/workspace" {
			return "", fmt.Errorf("file path cannot resolve to workspace directory")
		}
		if !strings.HasPrefix(cleaned, "/workspace/") {
			return "", fmt.Errorf("path must be inside /workspace")
		}
		return cleaned, nil
	}

	joined := filepath.Join("/workspace", cleaned)
	if joined == "/workspace" {
		return "", fmt.Errorf("file path cannot resolve to workspace directory")
	}
	if !strings.HasPrefix(joined, "/workspace/") {
		return "", fmt.Errorf("path must be inside /workspace")
	}
	return joined, nil
}
