package chromebridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
)

type sessionStatusTool struct {
	shared.BaseTool
	bridge *Bridge
}

// NewBrowserSessionStatus reports whether the local Chrome extension bridge is connected
// and lists open tabs when available.
func NewBrowserSessionStatus(bridge *Bridge) tools.ToolExecutor {
	return &sessionStatusTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "browser_session_status",
				Description: `Show the connection status for the local Chrome Extension Bridge.

If connected, also returns a list of open tabs from your existing Chrome profile.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"wait_seconds": {
							Type:        "integer",
							Description: "Optional: wait up to N seconds for the extension to connect before returning (default: 0).",
						},
						"max_tabs": {
							Type:        "integer",
							Description: "Optional: limit how many tabs to return when connected (default: 10, max: 50).",
						},
					},
				},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
			ports.ToolMetadata{
				Name:     "browser_session_status",
				Version:  "0.1.0",
				Category: "web",
				Tags:     []string{"browser", "chrome", "session"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
		),
		bridge: bridge,
	}
}

type chromeTab struct {
	TabID    int    `json:"tabId"`
	WindowID int    `json:"windowId"`
	URL      string `json:"url,omitempty"`
	Title    string `json:"title,omitempty"`
	Active   bool   `json:"active"`
}

func (t *sessionStatusTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.bridge == nil {
		err := errors.New("chrome extension bridge not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	maxTabs := 10
	if raw, ok := call.Arguments["max_tabs"].(float64); ok {
		maxTabs = int(raw)
	}
	if maxTabs <= 0 {
		maxTabs = 10
	}
	if maxTabs > 50 {
		maxTabs = 50
	}

	waitSeconds := 0
	if raw, ok := call.Arguments["wait_seconds"].(float64); ok {
		waitSeconds = int(raw)
	}
	if waitSeconds < 0 {
		waitSeconds = 0
	}

	if err := t.bridge.Start(); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	var waitErr error
	if waitSeconds > 0 && !t.bridge.Connected() {
		waitErr = t.bridge.WaitForConnected(ctx, time.Duration(waitSeconds)*time.Second)
	}

	client, version := t.bridge.LastHello()
	connected := t.bridge.Connected()
	status := map[string]any{
		"bridge": map[string]any{
			"listen_addr": t.bridge.Addr(),
			"connected":   connected,
			"client":      client,
			"version":     version,
			"capabilities": []string{
				"bridge.ping",
				"tabs.list",
				"cookies.getAll",
				"cookies.toHeader",
				"storage.getLocal",
			},
		},
		"tabs": []chromeTab{},
	}
	if waitErr != nil {
		status["wait_error"] = waitErr.Error()
	}

	if connected {
		raw, err := t.bridge.Call(ctx, "tabs.list", map[string]any{})
		if err != nil {
			status["tabs_error"] = err.Error()
		} else {
			var tabs []chromeTab
			if err := json.Unmarshal(raw, &tabs); err != nil {
				status["tabs_error"] = fmt.Sprintf("parse tabs.list result: %v", err)
			} else if len(tabs) > maxTabs {
				status["tabs"] = tabs[:maxTabs]
			} else {
				status["tabs"] = tabs
			}
		}
	}

	payload, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payload),
		Metadata: map[string]any{"browser_session_status": status},
	}, nil
}

type cookiesTool struct {
	shared.BaseTool
	bridge *Bridge
}

// NewBrowserCookies reads cookies from the connected Chrome profile via the extension bridge.
func NewBrowserCookies(bridge *Bridge) tools.ToolExecutor {
	return &cookiesTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "browser_cookies",
				Description: `Read cookies from your existing Chrome session via the Chrome Extension Bridge.

This is intended for reusing an already-logged-in browser profile (e.g., XHS) without remote-debugging port.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"domain": {
							Type:        "string",
							Description: "Cookie domain filter (e.g., xiaohongshu.com).",
						},
						"format": {
							Type:        "string",
							Description: "Output format: header|json (default: header).",
							Enum:        []any{"header", "json"},
						},
						"url": {
							Type:        "string",
							Description: "Optional: cookie URL scope (passed to chrome.cookies.getAll).",
						},
						"name": {
							Type:        "string",
							Description: "Optional: cookie name filter (passed to chrome.cookies.getAll).",
						},
						"wait_seconds": {
							Type:        "integer",
							Description: "Optional: wait up to N seconds for the extension to connect before reading cookies (default: 0).",
						},
					},
					Required: []string{"domain"},
				},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
			ports.ToolMetadata{
				Name:      "browser_cookies",
				Version:   "0.1.0",
				Category:  "web",
				Tags:      []string{"browser", "chrome", "cookies", "session"},
				Dangerous: true,
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
		),
		bridge: bridge,
	}
}

type chromeCookie struct {
	Name      string  `json:"name"`
	Value     string  `json:"value"`
	Domain    string  `json:"domain"`
	Path      string  `json:"path"`
	Expires   float64 `json:"expirationDate,omitempty"`
	Secure    bool    `json:"secure"`
	HTTPOnly  bool    `json:"httpOnly"`
	SameSite  string  `json:"sameSite,omitempty"`
	HostOnly  bool    `json:"hostOnly,omitempty"`
	Session   bool    `json:"session,omitempty"`
	StoreID   string  `json:"storeId,omitempty"`
	Partition string  `json:"partitionKey,omitempty"`
}

func (t *cookiesTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.bridge == nil {
		err := errors.New("chrome extension bridge not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	domain, _ := call.Arguments["domain"].(string)
	domain = strings.TrimSpace(domain)
	if domain == "" {
		err := errors.New("domain is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	format := "header"
	if raw, ok := call.Arguments["format"].(string); ok {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			format = strings.ToLower(trimmed)
		}
	}

	waitSeconds := 0
	if raw, ok := call.Arguments["wait_seconds"].(float64); ok {
		waitSeconds = int(raw)
	}
	if waitSeconds < 0 {
		waitSeconds = 0
	}

	if err := t.bridge.Start(); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if waitSeconds > 0 {
		if err := t.bridge.WaitForConnected(ctx, time.Duration(waitSeconds)*time.Second); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	switch format {
	case "header":
		rawHeader, err := t.bridge.Call(ctx, "cookies.toHeader", map[string]any{"domain": domain})
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		var header string
		if err := json.Unmarshal(rawHeader, &header); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		out := map[string]any{
			"domain":        domain,
			"header_name":   "Cookie",
			"cookie_header": header,
		}
		payload, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		return &ports.ToolResult{
			CallID:   call.ID,
			Content:  string(payload),
			Metadata: map[string]any{"browser_cookies": out},
		}, nil
	case "json":
		params := map[string]any{"domain": domain}
		if url, ok := call.Arguments["url"].(string); ok {
			if trimmed := strings.TrimSpace(url); trimmed != "" {
				params["url"] = trimmed
			}
		}
		if name, ok := call.Arguments["name"].(string); ok {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				params["name"] = trimmed
			}
		}

		rawCookies, err := t.bridge.Call(ctx, "cookies.getAll", params)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		var cookies []chromeCookie
		if err := json.Unmarshal(rawCookies, &cookies); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		rawHeader, err := t.bridge.Call(ctx, "cookies.toHeader", map[string]any{"domain": domain})
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		var header string
		if err := json.Unmarshal(rawHeader, &header); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		out := map[string]any{
			"domain":        domain,
			"header_name":   "Cookie",
			"cookie_header": header,
			"cookies":       cookies,
		}
		payload, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		return &ports.ToolResult{
			CallID:   call.ID,
			Content:  string(payload),
			Metadata: map[string]any{"browser_cookies": out},
		}, nil
	default:
		err := fmt.Errorf("unsupported format %q (use header|json)", format)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

type storageLocalTool struct {
	shared.BaseTool
	bridge *Bridge
}

// NewBrowserStorageLocal reads localStorage keys from a given tab via the extension bridge.
func NewBrowserStorageLocal(bridge *Bridge) tools.ToolExecutor {
	return &storageLocalTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "browser_storage_local",
				Description: `Read localStorage values from a specific Chrome tab via the Chrome Extension Bridge.

This tool injects a script into the target tab (requires host_permissions for that site).`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"tab_id": {
							Type:        "integer",
							Description: "Target tab id.",
						},
						"keys": {
							Type:        "array",
							Description: "List of localStorage keys to read.",
							Items:       &ports.Property{Type: "string"},
						},
						"wait_seconds": {
							Type:        "integer",
							Description: "Optional: wait up to N seconds for the extension to connect before reading storage (default: 0).",
						},
					},
					Required: []string{"tab_id", "keys"},
				},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
			ports.ToolMetadata{
				Name:      "browser_storage_local",
				Version:   "0.1.0",
				Category:  "web",
				Tags:      []string{"browser", "chrome", "storage", "session"},
				Dangerous: true,
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"application/json", "text/plain"},
				},
			},
		),
		bridge: bridge,
	}
}

func (t *storageLocalTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.bridge == nil {
		err := errors.New("chrome extension bridge not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	rawTabID, ok := call.Arguments["tab_id"].(float64)
	if !ok {
		err := errors.New("tab_id is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	tabID := int(rawTabID)
	if tabID <= 0 {
		err := fmt.Errorf("invalid tab_id %d", tabID)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	rawKeys, ok := call.Arguments["keys"].([]any)
	if !ok || len(rawKeys) == 0 {
		err := errors.New("keys is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	keys := make([]string, 0, len(rawKeys))
	for _, item := range rawKeys {
		key, ok := item.(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	if len(keys) == 0 {
		err := errors.New("keys must contain at least one string")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	waitSeconds := 0
	if raw, ok := call.Arguments["wait_seconds"].(float64); ok {
		waitSeconds = int(raw)
	}
	if waitSeconds < 0 {
		waitSeconds = 0
	}

	if err := t.bridge.Start(); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if waitSeconds > 0 {
		if err := t.bridge.WaitForConnected(ctx, time.Duration(waitSeconds)*time.Second); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	params := map[string]any{
		"tabId": tabID,
		"keys":  keys,
	}
	raw, err := t.bridge.Call(ctx, "storage.getLocal", params)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	out := map[string]any{
		"tab_id": tabID,
		"values": values,
	}
	payload, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payload),
		Metadata: map[string]any{"browser_storage_local": out},
	}, nil
}
