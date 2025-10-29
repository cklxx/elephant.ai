package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"fmt"
	"os"
	"strings"

	api "github.com/agent-infra/sandbox-sdk-go"
)

type listFiles struct {
        mode    tools.ExecutionMode
        sandbox *tools.SandboxManager
}

func NewListFiles(cfg FileToolConfig) ports.ToolExecutor {
        mode := cfg.Mode
        if mode == tools.ExecutionModeUnknown {
                mode = tools.ExecutionModeLocal
        }
        return &listFiles{mode: mode, sandbox: cfg.SandboxManager}
}

func (t *listFiles) Mode() tools.ExecutionMode {
        return t.mode
}

func (t *listFiles) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		path = "."
	}

	if t.mode == tools.ExecutionModeSandbox {
		return t.executeSandbox(ctx, call, path)
	}
	return t.executeLocal(call, path), nil
}

func (t *listFiles) executeLocal(call ports.ToolCall, path string) *ports.ToolResult {
	entries, err := os.ReadDir(path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}
	}

	var result strings.Builder
	for _, entry := range entries {
		info, _ := entry.Info()
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %s\n", entry.Name()))
		} else {
			result.WriteString(fmt.Sprintf("[FILE] %s (%d bytes)\n", entry.Name(), info.Size()))
		}
	}
	return &ports.ToolResult{CallID: call.ID, Content: result.String()}
}

func (t *listFiles) executeSandbox(ctx context.Context, call ports.ToolCall, path string) (*ports.ToolResult, error) {
	if t.sandbox == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("sandbox manager is required")}, nil
	}
	if err := t.sandbox.Initialize(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}

	resp, err := t.sandbox.File().ListPath(ctx, &api.FileListRequest{Path: path})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}
	data := resp.GetData()
	if data == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("sandbox returned empty response")}, nil
	}

	var result strings.Builder
	for _, entry := range data.GetFiles() {
		if entry == nil {
			continue
		}
		if entry.IsDirectory {
			result.WriteString(fmt.Sprintf("[DIR]  %s\n", entry.Name))
		} else {
			size := 0
			if entry.Size != nil {
				size = *entry.Size
			}
			result.WriteString(fmt.Sprintf("[FILE] %s (%d bytes)\n", entry.Name, size))
		}
	}

	return &ports.ToolResult{CallID: call.ID, Content: result.String()}, nil
}

func (t *listFiles) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "list_files",
		Description: "List files and directories",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path": {Type: "string", Description: "Directory path (default: .)"},
			},
		},
	}
}

func (t *listFiles) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "list_files", Version: "1.0.0", Category: "file_operations",
	}
}
