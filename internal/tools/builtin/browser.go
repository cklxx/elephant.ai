package builtin

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/tools"

	"github.com/agent-infra/sandbox-sdk-go/option"
)

type browserTool struct {
	config     BrowserToolConfig
	httpClient *http.Client
}

// NewBrowser creates the sandbox-only browser tool for capturing screenshots.
func NewBrowser(cfg BrowserToolConfig) ports.ToolExecutor {
	mode := cfg.Mode
	if mode == tools.ExecutionModeUnknown {
		mode = tools.ExecutionModeSandbox
	}

	return &browserTool{
		config: BrowserToolConfig{
			Mode:           mode,
			SandboxManager: cfg.SandboxManager,
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *browserTool) Mode() tools.ExecutionMode {
	return t.config.Mode
}

func (t *browserTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "browser",
		Version:  "1.0.0",
		Category: "web",
		Tags:     []string{"browser", "screenshot", "sandbox"},
	}
}

func (t *browserTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "browser",
		Description: `Capture a sandbox browser screenshot for the given URL.

Features:
- Loads the page inside the sandbox browser and captures a PNG screenshot
- Returns the page HTML for preview in the console
- Requires sandbox execution mode`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"url": {
					Type:        "string",
					Description: "Full URL to open in the sandbox browser",
				},
			},
			Required: []string{"url"},
		},
	}
}

func (t *browserTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.config.Mode != tools.ExecutionModeSandbox || t.config.SandboxManager == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("browser tool requires sandbox mode")}, nil
	}

	rawURL, _ := call.Arguments["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("url parameter required")}, nil
	}

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("invalid url: %w", err)}, nil
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("unsupported url scheme: %s", parsed.Scheme)}, nil
	}
	finalURL := parsed.String()

	screenshot, screenshotErr := t.captureScreenshot(ctx, finalURL)
	html, htmlErr := t.fetchHTML(ctx, finalURL)

	// Build text content for LLM (success status + HTML)
	var textContent strings.Builder
	textContent.WriteString(fmt.Sprintf("Browser visited: %s\n", finalURL))

	if htmlErr != nil {
		textContent.WriteString(fmt.Sprintf("Failed to fetch HTML: %s\n", htmlErr.Error()))
	} else if html != "" {
		textContent.WriteString(fmt.Sprintf("HTML content:\n%s", html))
	} else {
		textContent.WriteString("No HTML content retrieved\n")
	}

	// Build metadata for frontend (screenshot info)
	metadata := map[string]any{
		"url":     finalURL,
		"success": htmlErr == nil,
	}
	if screenshot != "" {
		metadata["screenshot"] = screenshot
	}
	if screenshotErr != nil {
		metadata["screenshot_error"] = screenshotErr.Error()
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  textContent.String(),
		Metadata: metadata,
	}, nil
}

func (t *browserTool) captureScreenshot(ctx context.Context, url string) (string, error) {
	if err := t.config.SandboxManager.Initialize(ctx); err != nil {
		return "", err
	}

	browserClient := t.config.SandboxManager.Browser()
	if browserClient == nil {
		return "", fmt.Errorf("sandbox browser client unavailable")
	}

	screenshotCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	reader, err := browserClient.TakeScreenshot(screenshotCtx, option.WithBaseURL(url))
	if err != nil {
		return "", err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read screenshot: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty screenshot data")
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return "data:image/png;base64," + encoded, nil
}

func (t *browserTool) fetchHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "ALEX-Agent/1.0 (Sandbox Browser)")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return string(body), nil
}
