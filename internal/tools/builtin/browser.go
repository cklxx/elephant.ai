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

	sandboxbrowser "github.com/agent-infra/sandbox-sdk-go/browser"
	"github.com/chromedp/chromedp"
)

type browserTool struct {
	config     BrowserToolConfig
	httpClient *http.Client
	navigateFn func(ctx context.Context, client *sandboxbrowser.Client, url string) error
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
		navigateFn: navigateSandboxBrowser,
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

	if t.navigateFn != nil {
		if err := t.navigateFn(ctx, browserClient, url); err != nil {
			return "", fmt.Errorf("navigate browser: %w", err)
		}
	}

	screenshotCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	reader, err := browserClient.TakeScreenshot(screenshotCtx)
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

func navigateSandboxBrowser(ctx context.Context, client *sandboxbrowser.Client, targetURL string) error {
	if client == nil {
		return fmt.Errorf("sandbox browser client unavailable")
	}

	infoCtx, cancelInfo := context.WithTimeout(ctx, 5*time.Second)
	defer cancelInfo()

	info, err := client.GetBrowserInfo(infoCtx)
	if err != nil {
		return fmt.Errorf("browser info: %w", err)
	}
	if info == nil || info.GetData() == nil {
		return fmt.Errorf("browser info missing data")
	}

	cdpURL := info.GetData().GetCdpUrl()
	if strings.TrimSpace(cdpURL) == "" {
		return fmt.Errorf("browser info missing cdp url")
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, cdpURL)
	defer cancelAlloc()

	chromeCtx, cancelChrome := chromedp.NewContext(allocCtx)
	defer cancelChrome()

	navigateCtx, cancelNavigate := context.WithTimeout(chromeCtx, 15*time.Second)
	defer cancelNavigate()

	if err := chromedp.Run(navigateCtx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("navigate to %s: %w", targetURL, err)
	}

	select {
	case <-time.After(750 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
