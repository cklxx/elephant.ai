package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/tools"
)

type browserInfo struct {
	config BrowserToolConfig
}

// NewBrowserInfo creates the sandbox-only workflow.diagnostic.browser_info tool.
func NewBrowserInfo(cfg BrowserToolConfig) ports.ToolExecutor {
	mode := cfg.Mode
	if mode == tools.ExecutionModeUnknown {
		if cfg.SandboxManager != nil {
			mode = tools.ExecutionModeSandbox
		} else {
			mode = tools.ExecutionModeLocal
		}
	}

	return &browserInfo{config: BrowserToolConfig{
		Mode:           mode,
		SandboxManager: cfg.SandboxManager,
	}}
}

func (t *browserInfo) Mode() tools.ExecutionMode {
	return t.config.Mode
}

func (t *browserInfo) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:      "workflow.diagnostic.browser_info",
		Version:   "0.1.0",
		Category:  string(ports.CategoryWeb),
		Tags:      []string{"browser", "diagnostics", "sandbox"},
		Dangerous: false,
	}
}

func (t *browserInfo) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "workflow.diagnostic.browser_info",
		Description: "Retrieve sandbox browser diagnostics including connection details and viewport sizing.",
		Parameters: ports.ParameterSchema{
			Type:       "object",
			Properties: map[string]ports.Property{},
		},
	}
}

func (t *browserInfo) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.config.Mode != tools.ExecutionModeSandbox {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("workflow.diagnostic.browser_info is only available in sandbox mode")}, nil
	}
	if t.config.SandboxManager == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("sandbox manager is required")}, nil
	}
	if err := t.config.SandboxManager.Initialize(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}

	browserClient := t.config.SandboxManager.Browser()
	if browserClient == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("sandbox browser client unavailable")}, nil
	}

	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	info, err := browserClient.GetBrowserInfo(infoCtx)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: tools.FormatSandboxError(err)}, nil
	}
	if info == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("browser did not return diagnostics")}, nil
	}

	payload := map[string]any{
		"captured_at": time.Now().UTC().Format(time.RFC3339),
	}

	if success := info.GetSuccess(); success != nil {
		payload["success"] = *success
	}
	if message := info.GetMessage(); message != nil {
		payload["message"] = *message
	}
	if data := info.GetData(); data != nil {
		payload["user_agent"] = data.GetUserAgent()
		payload["cdp_url"] = data.GetCdpUrl()
		payload["vnc_url"] = data.GetVncUrl()
		if viewport := data.GetViewport(); viewport != nil {
			payload["viewport_width"] = viewport.GetWidth()
			payload["viewport_height"] = viewport.GetHeight()
		}
	}

	contentBytes, err := json.Marshal(payload)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to marshal browser diagnostics: %w", err)}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: string(contentBytes),
		Metadata: map[string]any{
			"workflow.diagnostic.browser_info": payload,
		},
	}, nil
}
