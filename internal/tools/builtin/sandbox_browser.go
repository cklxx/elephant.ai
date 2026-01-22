package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	materialports "alex/internal/materials/ports"
	"alex/internal/sandbox"
)

type sandboxBrowserTool struct {
	client   *sandbox.Client
	vision   ports.ToolExecutor
	prompt   string
	uploader materialports.Migrator
}

type sandboxBrowserInfoTool struct {
	client *sandbox.Client
}

type sandboxBrowserScreenshotTool struct {
	client   *sandbox.Client
	vision   ports.ToolExecutor
	prompt   string
	uploader materialports.Migrator
}

type sandboxResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func NewSandboxBrowser(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxBrowserTool{
		client:   newSandboxClient(cfg),
		vision:   cfg.VisionTool,
		prompt:   cfg.VisionPrompt,
		uploader: cfg.AttachmentUploader,
	}
}

func NewSandboxBrowserInfo(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxBrowserInfoTool{client: newSandboxClient(cfg)}
}

func NewSandboxBrowserScreenshot(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxBrowserScreenshotTool{
		client:   newSandboxClient(cfg),
		vision:   cfg.VisionTool,
		prompt:   cfg.VisionPrompt,
		uploader: cfg.AttachmentUploader,
	}
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

Optional screenshot capture returns a PNG attachment. Prefer action logs and the live view; use capture_screenshot only when explicitly needed. For selector-based actions, use sandbox_browser_dom.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"actions": {
					Type:        "array",
					Description: "Ordered list of sandbox browser actions to execute.",
					Items:       &ports.Property{Type: "object"},
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
	var visionSummary string
	var visionMeta map[string]any
	if capture {
		name, _ := call.Arguments["screenshot_name"].(string)
		if name == "" {
			name = "sandbox_browser.png"
		}
		imageBytes, err := t.client.GetBytes(ctx, "/v1/browser/screenshot", call.SessionID)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		encoded := base64.StdEncoding.EncodeToString(imageBytes)
		attachments[name] = ports.Attachment{
			Name:      name,
			MediaType: "image/png",
			Data:      encoded,
			Source:    "sandbox_browser",
		}

		visionSummary, visionMeta = analyzeSandboxScreenshot(ctx, t.vision, t.prompt, call, encoded)
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
	if visionMeta != nil {
		metadata["vision"] = visionMeta
	}

	if visionSummary != "" {
		content = fmt.Sprintf("%s Vision summary: %s", content, visionSummary)
	}

	if len(attachments) > 0 {
		attachments = normalizeSandboxAttachments(ctx, attachments, t.uploader, "sandbox_browser")
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

	encoded := base64.StdEncoding.EncodeToString(imageBytes)
	attachment := ports.Attachment{
		Name:      name,
		MediaType: "image/png",
		Data:      encoded,
		Source:    "sandbox_browser_screenshot",
	}

	content := fmt.Sprintf("Captured screenshot [%s].", name)
	metadata := map[string]any{}
	visionSummary, visionMeta := analyzeSandboxScreenshot(ctx, t.vision, t.prompt, call, encoded)
	if visionSummary != "" {
		content = fmt.Sprintf("%s Vision summary: %s", content, visionSummary)
	}
	if visionMeta != nil {
		metadata["vision"] = visionMeta
	}

	attachments := map[string]ports.Attachment{name: attachment}
	if len(attachments) > 0 {
		attachments = normalizeSandboxAttachments(ctx, attachments, t.uploader, "sandbox_browser_screenshot")
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

const (
	httpMethodGet  = "GET"
	httpMethodPost = "POST"
)

const defaultSandboxVisionPrompt = "Describe the visible browser page. List key text, buttons, inputs, and any obvious next actions."

func analyzeSandboxScreenshot(ctx context.Context, vision ports.ToolExecutor, prompt string, call ports.ToolCall, encoded string) (string, map[string]any) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return "", nil
	}

	if vision == nil {
		summary := "Vision analysis unavailable (vision_analyze not configured)."
		return summary, map[string]any{"summary": summary, "error": "vision_analyze not configured"}
	}

	if strings.TrimSpace(prompt) == "" {
		prompt = defaultSandboxVisionPrompt
	}

	dataURI := "data:image/png;base64," + trimmed
	visionCall := ports.ToolCall{
		ID:           call.ID + ":vision",
		Name:         "vision_analyze",
		Arguments:    map[string]any{"images": []string{dataURI}, "prompt": prompt},
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
	}

	result, err := vision.Execute(ctx, visionCall)
	if err != nil {
		summary := summarizeVisionMessage(fmt.Sprintf("Vision analysis failed: %v", err))
		return summary, map[string]any{"summary": summary, "error": err.Error()}
	}
	if result == nil {
		summary := "Vision analysis returned no result."
		return summary, map[string]any{"summary": summary, "error": "empty result"}
	}

	if result.Error != nil {
		summary := summarizeVisionMessage(fmt.Sprintf("Vision analysis failed: %v", result.Error))
		metadata := map[string]any{"summary": summary, "error": result.Error.Error()}
		if result.Metadata != nil {
			metadata["metadata"] = result.Metadata
		}
		return summary, metadata
	}

	summary := summarizeVisionMessage(result.Content)
	if summary == "" {
		summary = "Vision analysis returned no text."
	}

	metadata := map[string]any{"summary": summary}
	if result.Metadata != nil {
		metadata["metadata"] = result.Metadata
	}
	return summary, metadata
}

func summarizeVisionMessage(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, "\n"); idx != -1 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	if len(trimmed) > 240 {
		return trimmed[:240] + "..."
	}
	return trimmed
}

func lastActionPerformed(responses []map[string]any) string {
	if len(responses) == 0 {
		return ""
	}
	last := responses[len(responses)-1]
	value, _ := last["action_performed"].(string)
	return value
}
