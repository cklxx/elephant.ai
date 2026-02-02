package browser

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/jsonx"
	"alex/internal/tools/builtin/shared"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

const defaultVisionPrompt = "Describe the visible browser page. List key text, buttons, inputs, and any obvious next actions."

func floatArgOptional(args map[string]any, key string) (float64, bool) {
	if args == nil {
		return 0, false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case jsonx.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func resolvePoint(ctx context.Context, sess *session, action map[string]any) (float64, float64, error) {
	var (
		x   float64
		y   float64
		err error
		ok  bool
	)

	if x, ok = floatArgOptional(action, "x"); ok {
		if y, ok = floatArgOptional(action, "y"); ok {
			x, y = applyOffset(action, x, y)
			return x, y, nil
		}
	}

	if selector := strings.TrimSpace(shared.StringArg(action, "selector")); selector != "" {
		x, y, err = resolvePointFromSelector(ctx, selector)
		if err != nil {
			return 0, 0, err
		}
		x, y = applyOffset(action, x, y)
		return x, y, nil
	}
	if selector := strings.TrimSpace(shared.StringArg(action, "target_selector")); selector != "" {
		x, y, err = resolvePointFromSelector(ctx, selector)
		if err != nil {
			return 0, 0, err
		}
		x, y = applyOffset(action, x, y)
		return x, y, nil
	}

	if sess != nil && sess.hasLastPos {
		x, y = applyOffset(action, sess.lastX, sess.lastY)
		return x, y, nil
	}
	if x, y, err = viewportCenter(ctx); err == nil {
		x, y = applyOffset(action, x, y)
		return x, y, nil
	}
	return 0, 0, errors.New("target point not specified")
}

func resolveDragTarget(ctx context.Context, sess *session, action map[string]any) (float64, float64, error) {
	if x, ok := floatArgOptional(action, "to_x"); ok {
		if y, ok := floatArgOptional(action, "to_y"); ok {
			return x, y, nil
		}
	}
	if x, ok := floatArgOptional(action, "end_x"); ok {
		if y, ok := floatArgOptional(action, "end_y"); ok {
			return x, y, nil
		}
	}
	if x, ok := floatArgOptional(action, "x2"); ok {
		if y, ok := floatArgOptional(action, "y2"); ok {
			return x, y, nil
		}
	}
	if selector := strings.TrimSpace(shared.StringArg(action, "to_selector")); selector != "" {
		return resolvePointFromSelector(ctx, selector)
	}
	if selector := strings.TrimSpace(shared.StringArg(action, "target_selector")); selector != "" {
		return resolvePointFromSelector(ctx, selector)
	}
	return resolvePoint(ctx, sess, action)
}

func resolvePointFromSelector(ctx context.Context, selector string) (float64, float64, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return 0, 0, errors.New("selector is required")
	}
	var nodes []*cdp.Node
	if err := chromedp.Run(ctx,
		chromedp.ScrollIntoView(selector, chromedp.ByQuery),
		chromedp.Nodes(selector, &nodes, chromedp.ByQuery),
	); err != nil {
		return 0, 0, err
	}
	if len(nodes) == 0 {
		return 0, 0, fmt.Errorf("selector %q not found", selector)
	}
	model, err := dom.GetBoxModel().WithNodeID(nodes[0].NodeID).Do(ctx)
	if err != nil {
		return 0, 0, err
	}
	if model == nil || len(model.Content) < 8 {
		return 0, 0, fmt.Errorf("selector %q has no box model", selector)
	}
	var sumX, sumY float64
	for i := 0; i < 8; i += 2 {
		sumX += model.Content[i]
		sumY += model.Content[i+1]
	}
	return sumX / 4, sumY / 4, nil
}

func applyOffset(action map[string]any, x, y float64) (float64, float64) {
	if dx, ok := floatArgOptional(action, "offset_x"); ok {
		x += dx
	} else if dx, ok := floatArgOptional(action, "offsetX"); ok {
		x += dx
	}
	if dy, ok := floatArgOptional(action, "offset_y"); ok {
		y += dy
	} else if dy, ok := floatArgOptional(action, "offsetY"); ok {
		y += dy
	}
	return x, y
}

func viewportCenter(ctx context.Context) (float64, float64, error) {
	var viewport struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`({width: window.innerWidth, height: window.innerHeight})`, &viewport)); err != nil {
		return 0, 0, err
	}
	if viewport.Width <= 0 || viewport.Height <= 0 {
		return 0, 0, errors.New("viewport size unavailable")
	}
	return viewport.Width / 2, viewport.Height / 2, nil
}

func parseMouseButton(action map[string]any, fallback input.MouseButton) input.MouseButton {
	raw := strings.TrimSpace(shared.StringArg(action, "button"))
	if raw == "" {
		raw = strings.TrimSpace(shared.StringArg(action, "mouse_button"))
	}
	switch strings.ToLower(raw) {
	case "left", "primary":
		return input.Left
	case "middle", "aux", "auxiliary":
		return input.Middle
	case "right", "secondary":
		return input.Right
	case "":
		if value, ok := floatArgOptional(action, "button"); ok {
			switch int(value) {
			case 0:
				return input.Left
			case 1:
				return input.Middle
			case 2:
				return input.Right
			}
		}
		return fallback
	default:
		return fallback
	}
}

func updateMousePosition(sess *session, x, y float64) {
	if sess == nil {
		return
	}
	sess.lastX = x
	sess.lastY = y
	sess.hasLastPos = true
}

func parseHotkey(action map[string]any) (string, []input.Modifier) {
	if action == nil {
		return "", nil
	}
	var tokens []string
	raw, ok := action["keys"]
	if ok && raw != nil {
		switch typed := raw.(type) {
		case []any:
			for _, item := range typed {
				if str, ok := item.(string); ok {
					tokens = append(tokens, str)
				}
			}
		case []string:
			tokens = append(tokens, typed...)
		case string:
			tokens = splitHotkeyString(typed)
		}
	}
	if len(tokens) == 0 {
		if key := strings.TrimSpace(shared.StringArg(action, "key")); key != "" {
			tokens = splitHotkeyString(key)
		}
	}
	if len(tokens) == 0 {
		return "", nil
	}

	var modifiers []input.Modifier
	var key string
	for _, token := range tokens {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}
		if mod, ok := parseModifier(trimmed); ok {
			modifiers = append(modifiers, mod)
			continue
		}
		key = trimmed
	}
	if key == "" {
		return "", modifiers
	}
	return normalizeKey(key), modifiers
}

func splitHotkeyString(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '+' || r == '-' || r == ' ' || r == ','
	})
	return parts
}

func parseModifier(value string) (input.Modifier, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "shift":
		return input.ModifierShift, true
	case "alt", "option":
		return input.ModifierAlt, true
	case "ctrl", "control":
		return input.ModifierCtrl, true
	case "meta", "cmd", "command", "super":
		return input.ModifierMeta, true
	default:
		return 0, false
	}
}

func parseModifiers(action map[string]any) input.Modifier {
	if action == nil {
		return 0
	}
	var mods []input.Modifier
	raw, ok := action["modifiers"]
	if ok && raw != nil {
		switch typed := raw.(type) {
		case []any:
			for _, item := range typed {
				if str, ok := item.(string); ok {
					if mod, ok := parseModifier(str); ok {
						mods = append(mods, mod)
					}
				}
			}
		case []string:
			for _, item := range typed {
				if mod, ok := parseModifier(item); ok {
					mods = append(mods, mod)
				}
			}
		case string:
			for _, item := range splitHotkeyString(typed) {
				if mod, ok := parseModifier(item); ok {
					mods = append(mods, mod)
				}
			}
		}
	}
	if shared.BoolArgWithDefault(action, "shift", false) {
		mods = append(mods, input.ModifierShift)
	}
	if shared.BoolArgWithDefault(action, "alt", false) {
		mods = append(mods, input.ModifierAlt)
	}
	if shared.BoolArgWithDefault(action, "ctrl", false) || shared.BoolArgWithDefault(action, "control", false) {
		mods = append(mods, input.ModifierCtrl)
	}
	if shared.BoolArgWithDefault(action, "meta", false) || shared.BoolArgWithDefault(action, "cmd", false) || shared.BoolArgWithDefault(action, "command", false) {
		mods = append(mods, input.ModifierMeta)
	}
	var combined input.Modifier
	for _, mod := range mods {
		combined |= mod
	}
	return combined
}

func normalizeKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	switch strings.ToLower(trimmed) {
	case "enter", "return":
		return kb.Enter
	case "esc", "escape":
		return kb.Escape
	case "tab":
		return kb.Tab
	case "backspace":
		return kb.Backspace
	case "delete", "del":
		return kb.Delete
	case "space", "spacebar":
		return " "
	case "arrowup", "up":
		return kb.ArrowUp
	case "arrowdown", "down":
		return kb.ArrowDown
	case "arrowleft", "left":
		return kb.ArrowLeft
	case "arrowright", "right":
		return kb.ArrowRight
	case "pageup":
		return kb.PageUp
	case "pagedown":
		return kb.PageDown
	case "home":
		return kb.Home
	case "end":
		return kb.End
	case "insert":
		return kb.Insert
	case "f1":
		return kb.F1
	case "f2":
		return kb.F2
	case "f3":
		return kb.F3
	case "f4":
		return kb.F4
	case "f5":
		return kb.F5
	case "f6":
		return kb.F6
	case "f7":
		return kb.F7
	case "f8":
		return kb.F8
	case "f9":
		return kb.F9
	case "f10":
		return kb.F10
	case "f11":
		return kb.F11
	case "f12":
		return kb.F12
	case "ctrl", "control":
		return kb.Control
	case "shift":
		return kb.Shift
	case "alt", "option":
		return kb.Alt
	case "meta", "cmd", "command", "super":
		return kb.Meta
	default:
		return trimmed
	}
}

func dispatchKeyEvent(ctx context.Context, key string, typ input.KeyType, modifiers input.Modifier) error {
	if key == "" {
		return errors.New("key is required")
	}
	runes := []rune(key)
	if len(runes) == 0 {
		return errors.New("key is required")
	}
	events := kb.Encode(runes[0])
	if len(events) == 0 {
		return errors.New("unsupported key")
	}
	var base *input.DispatchKeyEventParams
	switch typ {
	case input.KeyUp:
		base = events[len(events)-1]
	default:
		base = events[0]
	}
	params := *base
	params.Type = typ
	if modifiers != 0 {
		params.Modifiers |= modifiers
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return params.Do(ctx)
	}))
}

func captureScreenshot(ctx context.Context) ([]byte, error) {
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, errors.New("screenshot is empty")
	}
	return buf, nil
}

func analyzeScreenshot(ctx context.Context, vision tools.ToolExecutor, prompt string, call ports.ToolCall, encoded string) (string, map[string]any) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return "", nil
	}
	if vision == nil {
		summary := "Vision analysis unavailable (vision_analyze not configured)."
		return summary, map[string]any{"summary": summary, "error": "vision_analyze not configured"}
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = defaultVisionPrompt
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

func buildScreenshotAttachment(name string, imageBytes []byte, source string) ports.Attachment {
	encoded := base64.StdEncoding.EncodeToString(imageBytes)
	return ports.Attachment{
		Name:      name,
		MediaType: "image/png",
		Data:      encoded,
		Source:    source,
	}
}
