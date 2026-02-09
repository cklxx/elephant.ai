package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type browserScreenshotTool struct {
	shared.BaseTool
	manager *Manager
}

// NewBrowserScreenshot returns a local browser_screenshot tool backed by chromedp.
func NewBrowserScreenshot(manager *Manager) tools.ToolExecutor {
	return &browserScreenshotTool{
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
		manager: manager,
	}
}

func (t *browserScreenshotTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.manager == nil {
		err := errors.New("browser manager not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	session, err := t.manager.Session(call.SessionID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	name := strings.TrimSpace(shared.StringArg(call.Arguments, "name"))
	if name == "" {
		name = "browser.png"
	}

	var imageBytes []byte
	if err := session.withRunContext(ctx, t.manager.Config().timeoutOrDefault(), func(runCtx context.Context) error {
		var err error
		imageBytes, err = captureScreenshot(runCtx)
		return err
	}); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	attachment := buildScreenshotAttachment(name, imageBytes, "local_browser_screenshot")
	attachments := map[string]ports.Attachment{name: attachment}

	content := fmt.Sprintf("Captured screenshot [%s].", name)
	metadata := map[string]any{}
	cfg := t.manager.Config()
	visionSummary, visionMeta := analyzeScreenshot(ctx, cfg.VisionTool, cfg.VisionPrompt, call, attachment.Data)
	if visionSummary != "" {
		content = fmt.Sprintf("%s Vision summary: %s", content, visionSummary)
	}
	if visionMeta != nil {
		metadata["vision"] = visionMeta
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}
