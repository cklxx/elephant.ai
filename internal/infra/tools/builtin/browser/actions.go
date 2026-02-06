package browser

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

type browserAction struct {
	shared.BaseTool
	manager *Manager
}

func NewBrowserAction(manager *Manager) tools.ToolExecutor {
	return &browserAction{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "browser_action",
				Description: `Execute browser actions via the local browser instance.

Provide a list of action objects:
- action_type: MOVE_TO, CLICK, MOUSE_DOWN, MOUSE_UP, RIGHT_CLICK, DOUBLE_CLICK, DRAG_TO, SCROLL, TYPING, PRESS, KEY_DOWN, KEY_UP, HOTKEY
- additional fields vary per action type.

Optional screenshot capture returns a PNG attachment. Prefer action logs and the live view; use capture_screenshot only when explicitly needed. For selector-based actions, use browser_dom.`,
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
		manager: manager,
	}
}

func (t *browserAction) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
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

	if t.manager == nil {
		err := errors.New("browser manager not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	session, err := t.manager.Session(call.SessionID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	var responses []map[string]any
	var imageBytes []byte
	capture := shared.BoolArgWithDefault(call.Arguments, "capture_screenshot", false)
	if err := session.withRunContext(ctx, t.manager.Config().timeoutOrDefault(), func(runCtx context.Context) error {
		for _, action := range actions {
			actionType := normalizeActionType(action)
			if actionType == "" {
				err := errors.New("action_type is required")
				responses = append(responses, map[string]any{"action_performed": "unknown", "success": false, "error": err.Error()})
				return err
			}

			resp := map[string]any{"action_performed": actionType}
			if err := performBrowserAction(runCtx, session, actionType, action); err != nil {
				resp["success"] = false
				resp["error"] = err.Error()
				responses = append(responses, resp)
				return err
			}
			resp["success"] = true
			responses = append(responses, resp)
		}

		if capture {
			var err error
			imageBytes, err = captureScreenshot(runCtx)
			if err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err, Metadata: map[string]any{"browser_actions": responses}}, nil
	}

	attachments := map[string]ports.Attachment{}
	var visionSummary string
	var visionMeta map[string]any
	if capture && len(imageBytes) > 0 {
		name := strings.TrimSpace(shared.StringArg(call.Arguments, "screenshot_name"))
		if name == "" {
			name = "browser_action.png"
		}
		encoded := base64.StdEncoding.EncodeToString(imageBytes)
		attachments[name] = ports.Attachment{
			Name:      name,
			MediaType: "image/png",
			Data:      encoded,
			Source:    "local_browser",
		}
		cfg := t.manager.Config()
		visionSummary, visionMeta = analyzeScreenshot(ctx, cfg.VisionTool, cfg.VisionPrompt, call, encoded)
	}

	content := fmt.Sprintf("Executed %d browser action(s).", len(responses))
	if capture {
		content = fmt.Sprintf("%s Screenshot captured.", content)
	}
	if lastAction := lastActionPerformed(responses); lastAction != "" {
		content = fmt.Sprintf("%s Last action: %s.", content, lastAction)
	}
	if visionSummary != "" {
		content = fmt.Sprintf("%s Vision summary: %s", content, visionSummary)
	}

	metadata := map[string]any{
		"browser_actions": map[string]any{
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

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

func normalizeActionType(action map[string]any) string {
	actionType := strings.TrimSpace(shared.StringArg(action, "action_type"))
	if actionType == "" {
		actionType = strings.TrimSpace(shared.StringArg(action, "action"))
	}
	if actionType == "" {
		actionType = strings.TrimSpace(shared.StringArg(action, "type"))
	}
	return strings.ToUpper(actionType)
}

func performBrowserAction(ctx context.Context, sess *session, actionType string, action map[string]any) error {
	switch actionType {
	case "MOVE_TO":
		x, y, err := resolvePoint(ctx, sess, action)
		if err != nil {
			return err
		}
		if err := chromedp.Run(ctx, chromedp.MouseEvent(input.MouseMoved, x, y)); err != nil {
			return err
		}
		updateMousePosition(sess, x, y)
		return nil
	case "CLICK":
		return mouseClick(ctx, sess, action, input.Left, 1)
	case "RIGHT_CLICK":
		return mouseClick(ctx, sess, action, input.Right, 1)
	case "DOUBLE_CLICK":
		return mouseClick(ctx, sess, action, input.Left, 2)
	case "MOUSE_DOWN":
		return mouseButton(ctx, sess, action, input.MousePressed)
	case "MOUSE_UP":
		return mouseButton(ctx, sess, action, input.MouseReleased)
	case "DRAG_TO":
		return mouseDrag(ctx, sess, action)
	case "SCROLL":
		return mouseScroll(ctx, sess, action)
	case "TYPING":
		return keyType(ctx, sess, action)
	case "PRESS":
		return keyPress(ctx, action)
	case "KEY_DOWN":
		return keyEvent(ctx, action, input.KeyDown)
	case "KEY_UP":
		return keyEvent(ctx, action, input.KeyUp)
	case "HOTKEY":
		return keyHotkey(ctx, action)
	default:
		return fmt.Errorf("unsupported action_type %q", actionType)
	}
}

func mouseClick(ctx context.Context, sess *session, action map[string]any, button input.MouseButton, clicks int) error {
	x, y, err := resolvePoint(ctx, sess, action)
	if err != nil {
		return err
	}
	btn := parseMouseButton(action, button)
	opts := []chromedp.MouseOption{chromedp.ButtonType(btn), chromedp.ClickCount(clicks)}
	if err := chromedp.Run(ctx, chromedp.MouseClickXY(x, y, opts...)); err != nil {
		return err
	}
	updateMousePosition(sess, x, y)
	return nil
}

func mouseButton(ctx context.Context, sess *session, action map[string]any, typ input.MouseType) error {
	x, y, err := resolvePoint(ctx, sess, action)
	if err != nil {
		return err
	}
	btn := parseMouseButton(action, input.Left)
	opts := []chromedp.MouseOption{chromedp.ButtonType(btn)}
	if err := chromedp.Run(ctx, chromedp.MouseEvent(typ, x, y, opts...)); err != nil {
		return err
	}
	updateMousePosition(sess, x, y)
	return nil
}

func mouseDrag(ctx context.Context, sess *session, action map[string]any) error {
	startX, startY, err := resolvePoint(ctx, sess, action)
	if err != nil {
		return err
	}
	endX, endY, err := resolveDragTarget(ctx, sess, action)
	if err != nil {
		return err
	}
	btn := parseMouseButton(action, input.Left)
	press := chromedp.MouseEvent(input.MousePressed, startX, startY, chromedp.ButtonType(btn))
	move := chromedp.MouseEvent(input.MouseMoved, endX, endY, chromedp.ButtonType(btn))
	release := chromedp.MouseEvent(input.MouseReleased, endX, endY, chromedp.ButtonType(btn))
	if err := chromedp.Run(ctx, press, move, release); err != nil {
		return err
	}
	updateMousePosition(sess, endX, endY)
	return nil
}

func mouseScroll(ctx context.Context, sess *session, action map[string]any) error {
	x, y, err := resolvePoint(ctx, sess, action)
	if err != nil {
		return err
	}
	dx, _ := floatArgOptional(action, "dx")
	if dx == 0 {
		dx, _ = floatArgOptional(action, "delta_x")
	}
	dy, _ := floatArgOptional(action, "dy")
	if dy == 0 {
		dy, _ = floatArgOptional(action, "delta_y")
	}
	if dx == 0 && dy == 0 {
		return errors.New("scroll requires dx/dy")
	}
	opt := func(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
		return p.WithDeltaX(dx).WithDeltaY(dy)
	}
	if err := chromedp.Run(ctx, chromedp.MouseEvent(input.MouseWheel, x, y, opt)); err != nil {
		return err
	}
	updateMousePosition(sess, x, y)
	return nil
}

func keyType(ctx context.Context, _ *session, action map[string]any) error {
	text := strings.TrimSpace(shared.StringArg(action, "text"))
	if text == "" {
		text = strings.TrimSpace(shared.StringArg(action, "value"))
	}
	if text == "" {
		return errors.New("typing requires text")
	}
	if selector := strings.TrimSpace(shared.StringArg(action, "selector")); selector != "" {
		return chromedp.Run(ctx, chromedp.SendKeys(selector, text))
	}
	return chromedp.Run(ctx, chromedp.KeyEvent(text))
}

func keyPress(ctx context.Context, action map[string]any) error {
	key := strings.TrimSpace(shared.StringArg(action, "key"))
	if key == "" {
		return errors.New("press requires key")
	}
	return chromedp.Run(ctx, chromedp.KeyEvent(normalizeKey(key)))
}

func keyEvent(ctx context.Context, action map[string]any, typ input.KeyType) error {
	key := strings.TrimSpace(shared.StringArg(action, "key"))
	if key == "" {
		return errors.New("key event requires key")
	}
	modifiers := parseModifiers(action)
	return dispatchKeyEvent(ctx, normalizeKey(key), typ, modifiers)
}

func keyHotkey(ctx context.Context, action map[string]any) error {
	keys, modifiers := parseHotkey(action)
	if keys == "" {
		return errors.New("hotkey requires keys")
	}
	opts := []chromedp.KeyOption{}
	if len(modifiers) > 0 {
		opts = append(opts, chromedp.KeyModifiers(modifiers...))
	}
	return chromedp.Run(ctx, chromedp.KeyEvent(keys, opts...))
}

func lastActionPerformed(responses []map[string]any) string {
	if len(responses) == 0 {
		return ""
	}
	last := responses[len(responses)-1]
	value, _ := last["action_performed"].(string)
	return value
}
