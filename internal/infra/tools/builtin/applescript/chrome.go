package applescript

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

const (
	chromeProcessName  = "Google Chrome"
	defaultTabLimit    = 50
)

type chromeTool struct {
	shared.BaseTool
	runner Runner
}

// NewChrome creates a Chrome AppleScript tool using the real osascript runner.
func NewChrome() tools.ToolExecutor {
	return newChrome(ExecRunner{})
}

func newChrome(r Runner) *chromeTool {
	return &chromeTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "chrome",
				Description: `Control Google Chrome via AppleScript (macOS only).

Actions:
- list_tabs: list all open tabs across windows
- active_tab: get URL and title of the active tab
- open_url: open a URL in a new tab
- switch_tab: activate a specific tab by window/tab index
- navigate: navigate an existing tab to a new URL
- close_tab: close a specific tab`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: "Action to perform.",
							Enum:        []any{"list_tabs", "active_tab", "open_url", "switch_tab", "navigate", "close_tab"},
						},
						"url": {
							Type:        "string",
							Description: "URL to open or navigate to (required for open_url and navigate).",
						},
						"window_index": {
							Type:        "number",
							Description: "1-based window index (required for switch_tab, navigate, close_tab).",
						},
						"tab_index": {
							Type:        "number",
							Description: "1-based tab index (required for switch_tab, navigate, close_tab).",
						},
						"limit": {
							Type:        "number",
							Description: "Maximum number of tabs to return for list_tabs (default 50).",
						},
					},
					Required: []string{"action"},
				},
			},
			ports.ToolMetadata{
				Name:        "chrome",
				Version:     "0.1.0",
				Category:    "automation",
				Tags:        []string{"chrome", "browser", "macos", "applescript"},
				Dangerous:   false,
				SafetyLevel: ports.SafetyLevelReversible,
			},
		),
		runner: r,
	}
}

func (t *chromeTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action := shared.StringArg(call.Arguments, "action")
	if action == "" {
		return shared.ToolError(call.ID, "missing required parameter 'action'")
	}

	running, err := isAppRunning(ctx, t.runner, chromeProcessName)
	if err != nil {
		return shared.ToolError(call.ID, "failed to check if Chrome is running: %v", err)
	}
	if !running {
		return shared.ToolError(call.ID, "Google Chrome is not running")
	}

	switch action {
	case "list_tabs":
		return t.listTabs(ctx, call)
	case "active_tab":
		return t.activeTab(ctx, call)
	case "open_url":
		return t.openURL(ctx, call)
	case "switch_tab":
		return t.switchTab(ctx, call)
	case "navigate":
		return t.navigate(ctx, call)
	case "close_tab":
		return t.closeTab(ctx, call)
	default:
		return shared.ToolError(call.ID, "unsupported action: %s", action)
	}
}

func (t *chromeTool) listTabs(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	limit := defaultTabLimit
	if v, ok := shared.IntArg(call.Arguments, "limit"); ok && v > 0 {
		limit = v
	}

	script := `tell application "Google Chrome"
	set output to ""
	repeat with w from 1 to (count windows)
		repeat with t from 1 to (count tabs of window w)
			set tabURL to URL of tab t of window w
			set tabTitle to title of tab t of window w
			set output to output & w & "\t" & t & "\t" & tabURL & "\t" & tabTitle & linefeed
		end repeat
	end repeat
	return output
end tell`

	out, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "list_tabs: %v", err)
	}

	type tabEntry struct {
		Window int    `json:"window"`
		Tab    int    `json:"tab"`
		URL    string `json:"url"`
		Title  string `json:"title"`
	}

	var tabs []tabEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		w := parseInt(parts[0], 0)
		ti := parseInt(parts[1], 0)
		tabs = append(tabs, tabEntry{
			Window: w,
			Tab:    ti,
			URL:    parts[2],
			Title:  parts[3],
		})
		if len(tabs) >= limit {
			break
		}
	}

	data, _ := json.Marshal(tabs)
	return &ports.ToolResult{CallID: call.ID, Content: string(data)}, nil
}

func (t *chromeTool) activeTab(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	script := `tell application "Google Chrome"
	set tabURL to URL of active tab of front window
	set tabTitle to title of active tab of front window
	return tabURL & "\t" & tabTitle
end tell`

	out, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "active_tab: %v", err)
	}

	parts := strings.SplitN(out, "\t", 2)
	result := map[string]string{"url": parts[0]}
	if len(parts) > 1 {
		result["title"] = parts[1]
	}
	data, _ := json.Marshal(result)
	return &ports.ToolResult{CallID: call.ID, Content: string(data)}, nil
}

func (t *chromeTool) openURL(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawURL, errResult := shared.RequireStringArg(call.Arguments, call.ID, "url")
	if errResult != nil {
		return errResult, nil
	}
	if err := validateHTTPURL(rawURL); err != nil {
		return shared.ToolError(call.ID, "invalid url: %v", err)
	}

	escaped := escapeAppleScriptString(rawURL)
	script := fmt.Sprintf(`tell application "Google Chrome"
	tell front window
		make new tab with properties {URL:"%s"}
	end tell
	activate
end tell`, escaped)

	_, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "open_url: %v", err)
	}
	return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Opened %s in new tab", rawURL)}, nil
}

func (t *chromeTool) switchTab(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	w, wOK := shared.IntArg(call.Arguments, "window_index")
	ti, tOK := shared.IntArg(call.Arguments, "tab_index")
	if !wOK || !tOK || w < 1 || ti < 1 {
		return shared.ToolError(call.ID, "switch_tab requires positive 'window_index' and 'tab_index' (1-based)")
	}

	script := fmt.Sprintf(`tell application "Google Chrome"
	set active tab index of window %d to %d
	set index of window %d to 1
	activate
end tell`, w, ti, w)

	_, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "switch_tab: %v", err)
	}
	return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Switched to window %d, tab %d", w, ti)}, nil
}

func (t *chromeTool) navigate(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	w, wOK := shared.IntArg(call.Arguments, "window_index")
	ti, tOK := shared.IntArg(call.Arguments, "tab_index")
	if !wOK || !tOK || w < 1 || ti < 1 {
		return shared.ToolError(call.ID, "navigate requires positive 'window_index' and 'tab_index' (1-based)")
	}
	rawURL, errResult := shared.RequireStringArg(call.Arguments, call.ID, "url")
	if errResult != nil {
		return errResult, nil
	}
	if err := validateHTTPURL(rawURL); err != nil {
		return shared.ToolError(call.ID, "invalid url: %v", err)
	}

	escaped := escapeAppleScriptString(rawURL)
	script := fmt.Sprintf(`tell application "Google Chrome"
	set URL of tab %d of window %d to "%s"
end tell`, ti, w, escaped)

	_, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "navigate: %v", err)
	}
	return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Navigated window %d tab %d to %s", w, ti, rawURL)}, nil
}

func (t *chromeTool) closeTab(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	w, wOK := shared.IntArg(call.Arguments, "window_index")
	ti, tOK := shared.IntArg(call.Arguments, "tab_index")
	if !wOK || !tOK || w < 1 || ti < 1 {
		return shared.ToolError(call.ID, "close_tab requires positive 'window_index' and 'tab_index' (1-based)")
	}

	script := fmt.Sprintf(`tell application "Google Chrome"
	close tab %d of window %d
end tell`, ti, w)

	_, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "close_tab: %v", err)
	}
	return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Closed tab %d of window %d", ti, w)}, nil
}

// validateHTTPURL rejects non-http(s) schemes to prevent injection.
func validateHTTPURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only http/https URLs are allowed, got %q", u.Scheme)
	}
	return nil
}

func parseInt(s string, fallback int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
