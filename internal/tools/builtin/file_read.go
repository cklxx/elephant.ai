package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"fmt"
	"os"

	api "github.com/agent-infra/sandbox-sdk-go"
)

type fileRead struct {
        mode    tools.ExecutionMode
        sandbox *tools.SandboxManager
}

func NewFileRead(cfg FileToolConfig) ports.ToolExecutor {
        mode := cfg.Mode
        if mode == tools.ExecutionModeUnknown {
                mode = tools.ExecutionModeLocal
        }
        return &fileRead{mode: mode, sandbox: cfg.SandboxManager}
}

func (t *fileRead) Mode() tools.ExecutionMode {
        return t.mode
}

func (t *fileRead) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'path'")}, nil
	}

	if t.mode == tools.ExecutionModeSandbox {
		return t.executeSandbox(ctx, call, path)
	}
	return t.executeLocal(call, path), nil
}

func (t *fileRead) executeLocal(call ports.ToolCall, path string) *ports.ToolResult {
	content, err := os.ReadFile(path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}
	}
	return &ports.ToolResult{CallID: call.ID, Content: string(content)}
}

func (t *fileRead) executeSandbox(ctx context.Context, call ports.ToolCall, path string) (*ports.ToolResult, error) {
	if t.sandbox == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("sandbox manager is required")}, nil
	}
	if err := t.sandbox.Initialize(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}

	resp, err := t.sandbox.File().ReadFile(ctx, &api.FileReadRequest{File: path})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}
	data := resp.GetData()
	if data == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("sandbox returned empty response")}, nil
	}
	content := data.GetContent()
	return &ports.ToolResult{CallID: call.ID, Content: content}, nil
}

func (t *fileRead) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "file_read",
		Description: "Read file contents",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path": {Type: "string", Description: "File path"},
			},
			Required: []string{"path"},
		},
	}
}

func (t *fileRead) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "file_read", Version: "1.0.0", Category: "file_operations",
	}
}
