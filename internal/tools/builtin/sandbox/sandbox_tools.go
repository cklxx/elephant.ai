package sandbox

import (
	"context"
	"fmt"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	materialports "alex/internal/materials/ports"
	"alex/internal/sandbox"
)

type SandboxConfig struct {
	BaseURL            string
	VisionTool         tools.ToolExecutor
	VisionPrompt       string
	AttachmentUploader materialports.Migrator
}

func newSandboxClient(cfg SandboxConfig) *sandbox.Client {
	return sandbox.NewClient(sandbox.Config{BaseURL: cfg.BaseURL})
}

// doSandboxCall executes a sandbox API request and validates the response.
func doSandboxCall[T any](ctx context.Context, client *sandbox.Client, method, endpoint string, payload any, sessionID, opName string) (*T, error) {
	var response sandbox.Response[T]
	if err := client.DoJSON(ctx, method, endpoint, payload, sessionID, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("%s failed: %s", opName, response.Message)
	}
	if response.Data == nil {
		return nil, fmt.Errorf("%s returned empty payload", opName)
	}
	return response.Data, nil
}

// doSandboxRequest wraps doSandboxCall for tool Execute methods, returning a ToolResult on error.
func doSandboxRequest[T any](ctx context.Context, client *sandbox.Client, callID, sessionID, method, endpoint string, payload any, opName string) (*T, *ports.ToolResult) {
	data, err := doSandboxCall[T](ctx, client, method, endpoint, payload, sessionID, opName)
	if err != nil {
		return nil, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	return data, nil
}
