package browser

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveCDPURLReturnsWebSocketURLAsIs(t *testing.T) {
	got, err := resolveCDPURL(context.Background(), "ws://example/devtools/browser/abc")
	if err != nil {
		t.Fatalf("resolveCDPURL returned error: %v", err)
	}
	if got != "ws://example/devtools/browser/abc" {
		t.Fatalf("expected websocket url unchanged, got %q", got)
	}
}

func TestResolveCDPURLResolvesHTTPDevToolsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://resolved/devtools/browser/xyz"}`))
	}))
	t.Cleanup(server.Close)

	got, err := resolveCDPURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("resolveCDPURL returned error: %v", err)
	}
	if got != "ws://resolved/devtools/browser/xyz" {
		t.Fatalf("expected resolved websocket url, got %q", got)
	}

	hostPort := strings.TrimPrefix(server.URL, "http://")
	got, err = resolveCDPURL(context.Background(), hostPort)
	if err != nil {
		t.Fatalf("resolveCDPURL(hostPort) returned error: %v", err)
	}
	if got != "ws://resolved/devtools/browser/xyz" {
		t.Fatalf("expected resolved websocket url from hostPort, got %q", got)
	}

	_, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("split hostPort: %v", err)
	}
	got, err = resolveCDPURL(context.Background(), port)
	if err != nil {
		t.Fatalf("resolveCDPURL(port) returned error: %v", err)
	}
	if got != "ws://resolved/devtools/browser/xyz" {
		t.Fatalf("expected resolved websocket url from port, got %q", got)
	}
}

func TestResolveCDPURLErrorsOnEmptyWebSocketDebuggerURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":""}`))
	}))
	t.Cleanup(server.Close)

	_, err := resolveCDPURL(context.Background(), server.URL)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
