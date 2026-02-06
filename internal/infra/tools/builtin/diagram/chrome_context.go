package diagram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/sandbox"

	"github.com/chromedp/chromedp"
)

func (t *diagramRender) newChromeContext(ctx context.Context, call ports.ToolCall) (context.Context, func(), error) {
	switch t.mode {
	case "local":
		return t.newLocalChromeContext(ctx)
	case "sandbox":
		return t.newSandboxChromeContext(ctx, call.SessionID)
	default:
		return nil, nil, fmt.Errorf("unsupported diagram_render runtime %q", t.mode)
	}
}

func (t *diagramRender) newLocalChromeContext(ctx context.Context) (context.Context, func(), error) {
	if t.browserMgr == nil {
		return nil, nil, errors.New("browser manager not configured for diagram render")
	}
	return t.browserMgr.NewTemporaryContext(ctx)
}

func (t *diagramRender) newSandboxChromeContext(ctx context.Context, sessionID string) (context.Context, func(), error) {
	if t.sandboxClient == nil {
		return nil, nil, errors.New("sandbox client not configured")
	}
	info, err := fetchSandboxBrowserInfo(ctx, t.sandboxClient, sessionID)
	if err != nil {
		return nil, nil, err
	}
	cdpURL := strings.TrimSpace(info.CDPURL)
	if cdpURL == "" {
		return nil, nil, errors.New("sandbox browser CDP URL is missing")
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, cdpURL)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	closeFn := func() {
		chromeCancel()
		allocCancel()
	}
	return chromeCtx, closeFn, nil
}

func fetchSandboxBrowserInfo(ctx context.Context, client *sandbox.Client, sessionID string) (*sandbox.BrowserInfo, error) {
	var response sandbox.Response[sandbox.BrowserInfo]
	if err := client.DoJSON(ctx, http.MethodGet, "/v1/browser/info", nil, sessionID, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("sandbox browser info failed: %s", response.Message)
	}
	if response.Data == nil {
		return nil, errors.New("sandbox browser info returned empty data")
	}
	return response.Data, nil
}
