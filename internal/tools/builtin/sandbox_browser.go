package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"alex/internal/agent/ports"
	"alex/internal/sandbox"
)

type sandboxBrowserTool struct {
	client *sandbox.Client
}

type sandboxBrowserInfoTool struct {
	client *sandbox.Client
}

type sandboxBrowserScreenshotTool struct {
	client *sandbox.Client
}

type sandboxResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func NewSandboxBrowser(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxBrowserTool{client: newSandboxClient(cfg)}
}

func NewSandboxBrowserInfo(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxBrowserInfoTool{client: newSandboxClient(cfg)}
}

func NewSandboxBrowserScreenshot(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxBrowserScreenshotTool{client: newSandboxClient(cfg)}
}

func (t *sandboxBrowserTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "sandbox_browser",
		Version:  "0.1.0",
		Category: "web",
		Tags:     []string{"sandbox", "browser", "automation"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces:          []string{"text/plain"},
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *sandboxBrowserTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "sandbox_browser",
		Description: `Execute browser actions inside the AIO Sandbox.

Provide a list of action objects that match the sandbox browser API:
- action_type: MOVE_TO, CLICK, MOUSE_DOWN, MOUSE_UP, RIGHT_CLICK, DOUBLE_CLICK, DRAG_TO, SCROLL, TYPING, PRESS, KEY_DOWN, KEY_UP, HOTKEY
- additional fields vary per action (see sandbox OpenAPI).

Optional screenshot capture returns a PNG attachment.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"actions": {
					Type:        "array",
					Description: "Ordered list of sandbox browser actions to execute.",
				},
				"capture_screenshot": {
					Type:        "boolean",
					Description: "Capture a screenshot after executing all actions.",
				},
				"screenshot_name": {
					Type:        "string",
					Description: "Filename to use for the screenshot attachment.",
				},
			},
			Required: []string{"actions"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces:          []string{"text/plain"},
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *sandboxBrowserTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawActions, _ := call.Arguments["actions"].([]any)
	if len(rawActions) == 0 {
		err := errors.New("actions is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	actions := make([]map[string]any, 0, len(rawActions))
	for _, item := range rawActions {
		action, ok := item.(map[string]any)
		if !ok {
			err := errors.New("actions must be objects")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		actions = append(actions, action)
	}

	var responses []map[string]any
	for _, action := range actions {
		var response map[string]any
		if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/browser/actions", action, call.SessionID, &response); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		responses = append(responses, response)
	}

	attachments := map[string]ports.Attachment{}
	capture, _ := call.Arguments["capture_screenshot"].(bool)
	if capture {
		name, _ := call.Arguments["screenshot_name"].(string)
		if name == "" {
			name = "sandbox_browser.png"
		}
		imageBytes, err := t.client.GetBytes(ctx, "/v1/browser/screenshot", call.SessionID)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		attachments[name] = ports.Attachment{
			Name:      name,
			MediaType: "image/png",
			Data:      base64.StdEncoding.EncodeToString(imageBytes),
			Source:    "sandbox_browser",
		}
	}

	content := fmt.Sprintf("Executed %d sandbox browser action(s).", len(responses))
	if capture {
		content = fmt.Sprintf("%s Screenshot captured.", content)
	}
	if lastAction := lastActionPerformed(responses); lastAction != "" {
		content = fmt.Sprintf("%s Last action: %s.", content, lastAction)
	}

	metadata := map[string]any{
		"sandbox_browser": map[string]any{
			"actions":   actions,
			"responses": responses,
		},
	}

	if capture {
		metadata["screenshot"] = true
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

func (t *sandboxBrowserInfoTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "sandbox_browser_info",
		Version:  "0.1.0",
		Category: "web",
		Tags:     []string{"sandbox", "browser", "metadata"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"application/json", "text/plain"},
		},
	}
}

func (t *sandboxBrowserInfoTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "sandbox_browser_info",
		Description: "Fetch browser metadata from the AIO Sandbox (user agent, CDP URL, viewport, VNC URL).",
		Parameters: ports.ParameterSchema{
			Type:       "object",
			Properties: map[string]ports.Property{},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"application/json", "text/plain"},
		},
	}
}

func (t *sandboxBrowserInfoTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	var response sandboxResponse
	if err := t.client.DoJSON(ctx, httpMethodGet, "/v1/browser/info", nil, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox browser info failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload, err := json.MarshalIndent(response.Data, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payload),
		Metadata: map[string]any{"sandbox_browser_info": response.Data},
	}, nil
}

func (t *sandboxBrowserScreenshotTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "sandbox_browser_screenshot",
		Version:  "0.1.0",
		Category: "web",
		Tags:     []string{"sandbox", "browser", "screenshot"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *sandboxBrowserScreenshotTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "sandbox_browser_screenshot",
		Description: "Capture a screenshot from the sandbox browser.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"name": {
					Type:        "string",
					Description: "Filename to use for the screenshot attachment.",
				},
			},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *sandboxBrowserScreenshotTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	name, _ := call.Arguments["name"].(string)
	if name == "" {
		name = "sandbox_browser.png"
	}

	imageBytes, err := t.client.GetBytes(ctx, "/v1/browser/screenshot", call.SessionID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	attachment := ports.Attachment{
		Name:      name,
		MediaType: "image/png",
		Data:      base64.StdEncoding.EncodeToString(imageBytes),
		Source:    "sandbox_browser_screenshot",
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     fmt.Sprintf("Captured screenshot [%s].", name),
		Attachments: map[string]ports.Attachment{name: attachment},
	}, nil
}

const (
	httpMethodGet  = "GET"
	httpMethodPost = "POST"
)

func lastActionPerformed(responses []map[string]any) string {
	if len(responses) == 0 {
		return ""
	}
	last := responses[len(responses)-1]
	value, _ := last["action_performed"].(string)
	return value
}
