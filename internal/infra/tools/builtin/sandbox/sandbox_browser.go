package sandbox

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	materialports "alex/internal/domain/materials/ports"
	"alex/internal/infra/sandbox"
	"alex/internal/infra/tools/builtin/shared"
)

type sandboxBrowserTool struct {
	shared.BaseTool
	client   *sandbox.Client
	vision   tools.ToolExecutor
	prompt   string
	uploader materialports.Migrator
}

type sandboxBrowserInfoTool struct {
	shared.BaseTool
	client *sandbox.Client
}

type sandboxBrowserScreenshotTool struct {
	shared.BaseTool
	client   *sandbox.Client
	vision   tools.ToolExecutor
	prompt   string
	uploader materialports.Migrator
}

type sandboxResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func NewSandboxBrowser(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxBrowserTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "browser_action",
				Description: `Execute browser actions via the local browser instance.

Provide a list of action objects:
- action_type: MOVE_TO, CLICK, MOUSE_DOWN, MOUSE_UP, RIGHT_CLICK, DOUBLE_CLICK, DRAG_TO, SCROLL, TYPING, PRESS, KEY_DOWN, KEY_UP, HOTKEY
- additional fields vary per action type.

Optional screenshot capture returns a PNG attachment. Prefer action logs and the live view; use capture_screenshot only when explicitly needed. For selector-based actions, use browser_dom.
Do not use browser_action when the task is only to inspect URL/title/session metadata; use browser_info.
Do not use browser_action for deterministic computation/recalculation tasks; use execute_code.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"actions": {
							Type:        "array",
							Description: "Ordered list of browser actions to execute.",
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
			},
			ports.ToolMetadata{
				Name:     "browser_action",
				Version:  "0.1.0",
				Category: "web",
				Tags:     []string{"browser", "automation", "interaction"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces:          []string{"text/plain"},
					ProducesArtifacts: []string{"image/png"},
				},
			},
		),
		client:   newSandboxClient(cfg),
		vision:   cfg.VisionTool,
		prompt:   cfg.VisionPrompt,
		uploader: cfg.AttachmentUploader,
	}
}

func NewSandboxBrowserInfo(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxBrowserInfoTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "browser_info",
				Description: "Inspect browser tab/session state metadata (current URL, title, viewport, user agent). Use for read-only inspection/state checks; use browser_action or browser_dom for interactions.",
				Parameters: ports.ParameterSchema{
					Type:       "object",
					Properties: map[string]ports.Property{},
				},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
			ports.ToolMetadata{
				Name:     "browser_info",
				Version:  "0.1.0",
				Category: "web",
				Tags:     []string{"browser", "metadata", "info", "session", "tab", "url", "state"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
		),
		client: newSandboxClient(cfg),
	}
}

func NewSandboxBrowserScreenshot(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxBrowserScreenshotTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "browser_screenshot",
				Description: "Capture visual screenshot (PNG) from the current browser page. Use only when image capture is explicitly required; do not use for semantic text retrieval or downloadable file-package delivery.",
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
			},
			ports.ToolMetadata{
				Name:     "browser_screenshot",
				Version:  "0.1.0",
				Category: "web",
				Tags:     []string{"browser", "screenshot", "capture"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					ProducesArtifacts: []string{"image/png"},
				},
			},
		),
		client:   newSandboxClient(cfg),
		vision:   cfg.VisionTool,
		prompt:   cfg.VisionPrompt,
		uploader: cfg.AttachmentUploader,
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

func analyzeSandboxScreenshot(ctx context.Context, vision tools.ToolExecutor, prompt string, call ports.ToolCall, encoded string) (string, map[string]any) {
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
