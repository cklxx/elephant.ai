package browser

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"

	"github.com/chromedp/chromedp"
)

type browserInfoTool struct {
	shared.BaseTool
	manager *Manager
}

// NewBrowserInfo returns a local browser_info tool backed by chromedp.
func NewBrowserInfo(manager *Manager) tools.ToolExecutor {
	return &browserInfoTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "browser_info",
				Description: "Fetch browser metadata (user agent, viewport, URL, title).",
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
				Tags:     []string{"browser", "metadata", "info"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
		),
		manager: manager,
	}
}

func (t *browserInfoTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.manager == nil {
		err := errors.New("browser manager not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	session, err := t.manager.Session(call.SessionID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	info := map[string]any{}
	if err := session.withRunContext(ctx, t.manager.Config().timeoutOrDefault(), func(runCtx context.Context) error {
		var userAgent string
		var url string
		var title string
		var viewport struct {
			Width            float64 `json:"width"`
			Height           float64 `json:"height"`
			DevicePixelRatio float64 `json:"device_pixel_ratio"`
		}
		if err := chromedp.Run(runCtx,
			chromedp.Evaluate(`navigator.userAgent`, &userAgent),
			chromedp.Location(&url),
			chromedp.Title(&title),
			chromedp.Evaluate(`({width: window.innerWidth, height: window.innerHeight, devicePixelRatio: window.devicePixelRatio})`, &viewport),
		); err != nil {
			return err
		}
		if ua := strings.TrimSpace(userAgent); ua != "" {
			info["user_agent"] = ua
		}
		if url = strings.TrimSpace(url); url != "" {
			info["url"] = url
		}
		if title = strings.TrimSpace(title); title != "" {
			info["title"] = title
		}
		if viewport.Width > 0 && viewport.Height > 0 {
			info["viewport"] = map[string]any{
				"width":              viewport.Width,
				"height":             viewport.Height,
				"device_pixel_ratio": viewport.DevicePixelRatio,
			}
		}
		return nil
	}); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payload),
		Metadata: map[string]any{"browser_info": info},
	}, nil
}
