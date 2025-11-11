package builtin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/tools"

	sandboxbrowser "github.com/agent-infra/sandbox-sdk-go/browser"
)

func TestBrowserExecuteSuccess(t *testing.T) {
	stub := newBrowserTransportStub(t)

	htmlClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: stub,
	}

	originalDefaultTransport := http.DefaultTransport
	http.DefaultTransport = stub
	t.Cleanup(func() {
		http.DefaultTransport = originalDefaultTransport
	})

	sandbox := tools.NewSandboxManager("http://sandbox.test")

	tool := NewBrowser(BrowserToolConfig{
		Mode:           tools.ExecutionModeSandbox,
		SandboxManager: sandbox,
	})

	browserImpl, ok := tool.(*browserTool)
	if !ok {
		t.Fatalf("unexpected browser tool implementation: %T", tool)
	}
	browserImpl.httpClient = htmlClient
	browserImpl.navigateFn = func(ctx context.Context, client *sandboxbrowser.Client, navURL string) error {
		stub.navigateCalls = append(stub.navigateCalls, navURL)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	targetURL := "http://page.test/dashboard/reports?id=campaign-123&view=detail"
	call := ports.ToolCall{
		ID:   "browser-success",
		Name: "browser",
		Arguments: map[string]any{
			"url": targetURL,
		},
	}

	result, execErr := tool.Execute(ctx, call)
	if execErr != nil {
		t.Fatalf("Execute failed: %v", execErr)
	}
	if result.Error != nil {
		t.Fatalf("tool returned error: %v", result.Error)
	}

	meta := result.Metadata
	if success, ok := meta["success"].(bool); !ok || !success {
		t.Fatalf("expected success metadata, got: %#v", meta["success"])
	}
	if errMeta, ok := meta["screenshot_error"]; ok && errMeta != nil {
		t.Fatalf("unexpected screenshot error: %v", errMeta)
	}

	screenshot, ok := meta["screenshot"].(string)
	if !ok || !strings.HasPrefix(screenshot, "data:image/png;base64,") {
		t.Fatalf("expected base64 PNG screenshot, got %#v", meta["screenshot"])
	}

	if !strings.Contains(result.Content, "Browser visited: "+targetURL) {
		t.Errorf("missing visit log in content: %q", result.Content)
	}
	if !strings.Contains(result.Content, "campaign-123") {
		t.Errorf("expected unique identifier in HTML content, got: %q", result.Content)
	}

	if len(result.Attachments) != 2 {
		t.Fatalf("expected screenshot and HTML attachments, got %d", len(result.Attachments))
	}

	var (
		screenshotAttachment *ports.Attachment
		htmlAttachment       *ports.Attachment
	)
	for name, att := range result.Attachments {
		switch {
		case strings.HasSuffix(name, ".png"):
			copy := att
			screenshotAttachment = &copy
		case strings.HasSuffix(name, ".html"):
			copy := att
			htmlAttachment = &copy
		}
	}
	if screenshotAttachment == nil {
		t.Fatalf("expected PNG attachment in result: %+v", result.Attachments)
	}
	if screenshotAttachment.MediaType != "image/png" {
		t.Fatalf("expected screenshot media type image/png, got %q", screenshotAttachment.MediaType)
	}
	if screenshotAttachment.URI != meta["screenshot"] {
		t.Fatalf("expected screenshot attachment URI to match metadata, got %q vs %q", screenshotAttachment.URI, meta["screenshot"])
	}
	if screenshotAttachment.Data == "" {
		t.Fatalf("expected screenshot attachment to include base64 payload")
	}

	if htmlAttachment == nil {
		t.Fatalf("expected HTML attachment in result: %+v", result.Attachments)
	}
	if htmlAttachment.MediaType != "text/html" {
		t.Fatalf("expected HTML media type, got %q", htmlAttachment.MediaType)
	}
	if htmlAttachment.Data == "" || !strings.HasPrefix(htmlAttachment.URI, "data:text/html;base64,") {
		t.Fatalf("expected HTML attachment data URI, got %+v", htmlAttachment)
	}

	if stub.shellExecCalls == 0 {
		t.Errorf("expected sandbox shell exec to be invoked")
	}
	if stub.screenshotCalls == 0 {
		t.Errorf("expected sandbox screenshot endpoint to be invoked")
	}
	if !stub.htmlServed {
		t.Errorf("expected HTML endpoint to be served")
	}
	if stub.lastPageURL == nil {
		t.Fatalf("expected page request URL to be captured")
	}
	if stub.lastPageURL.Path != "/dashboard/reports" || stub.lastPageURL.RawQuery != "id=campaign-123&view=detail" {
		t.Errorf("HTML fetch routed incorrectly: %s?%s", stub.lastPageURL.Path, stub.lastPageURL.RawQuery)
	}
	if len(stub.navigateCalls) != 1 || stub.navigateCalls[0] != targetURL {
		t.Fatalf("expected navigateFn to be invoked with %s, got %v", targetURL, stub.navigateCalls)
	}

	prefix := "data:image/png;base64,"
	data := strings.TrimPrefix(screenshot, prefix)
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatalf("failed to decode screenshot base64: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() == 0 || b.Dy() == 0 {
		t.Fatalf("decoded PNG has empty bounds: %v", b)
	}
}

type browserTransportStub struct {
	t               *testing.T
	pngData         []byte
	shellExecCalls  int
	screenshotCalls int
	htmlServed      bool
	lastPageURL     *url.URL
	navigateCalls   []string
}

func newBrowserTransportStub(t *testing.T) *browserTransportStub {
	t.Helper()
	return &browserTransportStub{
		t:       t,
		pngData: encodeStubPNG(t),
	}
}

func (s *browserTransportStub) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodPost && req.URL.Host == "sandbox.test" && req.URL.Path == "/v1/shell/exec":
		s.shellExecCalls++
		payload := map[string]any{
			"success": true,
			"data": map[string]any{
				"session_id": "stub-session",
				"command":    "echo 'alex-sandbox-health'",
				"status":     "completed",
				"output":     "alex-sandbox-health\n",
			},
		}
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(payload); err != nil {
			return nil, err
		}
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(buf.Bytes())),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil

	case req.Method == http.MethodGet && req.URL.Host == "sandbox.test" && strings.HasPrefix(req.URL.Path, "/v1/browser/screenshot"):
		s.screenshotCalls++
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(s.pngData)),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "image/png")
		return resp, nil

	case req.Method == http.MethodGet && req.URL.Host == "page.test":
		s.htmlServed = true
		clone := *req.URL
		s.lastPageURL = &clone
		html := fmt.Sprintf("<html><body>stub page %s</body></html>", req.URL.RawQuery)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(html)),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")
		return resp, nil
	}

	return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
}

func encodeStubPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 60, B: 90, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
