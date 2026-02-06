package web

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

func TestHTMLEditValidateOnlyInline(t *testing.T) {
	tool := NewHTMLEdit(nil, HTMLEditConfig{})
	html := "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Test</title></head><body><h1>Hello</h1></body></html>"

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"action":      "validate",
			"html":        html,
			"output_name": "sample.html",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}

	att, ok := result.Attachments["sample.html"]
	if !ok {
		t.Fatalf("expected attachment sample.html")
	}
	decoded, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Fatalf("failed to decode attachment data: %v", err)
	}
	if string(decoded) != html {
		t.Fatalf("attachment HTML mismatch")
	}

	validation, ok := result.Metadata["validation"].(map[string]any)
	if !ok {
		t.Fatalf("expected validation metadata")
	}
	if got := asInt(validation["error_count"]); got != 0 {
		t.Fatalf("expected 0 errors, got %d", got)
	}
}

func TestHTMLEditViewFromAttachment(t *testing.T) {
	tool := NewHTMLEdit(nil, HTMLEditConfig{})
	html := "<!DOCTYPE html><html><head><title>Demo</title></head><body>OK</body></html>"
	encoded := base64.StdEncoding.EncodeToString([]byte(html))
	attachments := map[string]ports.Attachment{
		"demo.html": {
			Name:      "demo.html",
			MediaType: "text/html",
			Data:      encoded,
		},
	}
	ctx := tools.WithAttachmentContext(context.Background(), attachments, nil)

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"action": "view",
			"name":   "demo.html",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if strings.TrimSpace(result.Content) != html {
		t.Fatalf("expected raw HTML content")
	}
}

func TestHTMLEditPrefersInlineHTMLOverURI(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>REMOTE</body></html>"))
	}))
	defer server.Close()

	tool := NewHTMLEdit(nil, HTMLEditConfig{})
	html := "<!DOCTYPE html><html><head><title>Demo</title></head><body>INLINE</body></html>"
	encoded := base64.StdEncoding.EncodeToString([]byte(html))
	attachments := map[string]ports.Attachment{
		"demo.html": {
			Name:      "demo.html",
			MediaType: "text/html",
			Data:      encoded,
			URI:       server.URL,
		},
	}
	ctx := tools.WithAttachmentContext(context.Background(), attachments, nil)

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-3",
		Arguments: map[string]any{
			"action": "view",
			"name":   "demo.html",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if strings.TrimSpace(result.Content) != html {
		t.Fatalf("expected inline HTML content")
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("expected inline payload to avoid HTTP fetch")
	}
}

func asInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
