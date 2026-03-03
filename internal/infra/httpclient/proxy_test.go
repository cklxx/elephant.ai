package httpclient

import (
	"net"
	"net/http"
	"sync"
	"testing"
)

func resetProxyState(t *testing.T) {
	t.Helper()
	proxyModeOnce = sync.Once{}
	resolvedProxyMode = proxyModeAuto
	localProxyBypassCache = sync.Map{}
	localProxyWarned = sync.Map{}
}

func TestProxyFuncAutoUsesReachableLoopbackProxy(t *testing.T) {
	resetProxyState(t)
	t.Setenv(proxyModeEnv, "auto")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	t.Setenv("HTTPS_PROXY", "http://"+listener.Addr().String())
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	proxy, err := proxyFunc(nil)(req)
	if err != nil {
		t.Fatalf("proxy func error: %v", err)
	}
	if proxy == nil {
		t.Fatalf("expected proxy to be returned")
	}
	if proxy.Host != listener.Addr().String() {
		t.Fatalf("expected proxy host %q, got %q", listener.Addr().String(), proxy.Host)
	}
}

func TestProxyFuncAutoBypassesUnreachableLoopbackProxy(t *testing.T) {
	resetProxyState(t)
	t.Setenv(proxyModeEnv, "auto")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	t.Setenv("HTTPS_PROXY", "http://"+addr)
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	proxy, err := proxyFunc(nil)(req)
	if err != nil {
		t.Fatalf("proxy func error: %v", err)
	}
	if proxy != nil {
		t.Fatalf("expected proxy to be bypassed, got %v", proxy)
	}
}

func TestProxyFuncStrictAlwaysReturnsProxy(t *testing.T) {
	resetProxyState(t)
	t.Setenv(proxyModeEnv, "strict")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	proxy, err := proxyFunc(nil)(req)
	if err != nil {
		t.Fatalf("proxy func error: %v", err)
	}
	if proxy == nil {
		t.Fatalf("expected strict proxy mode to return proxy")
	}
}

func TestProxyFuncDirectAlwaysReturnsNil(t *testing.T) {
	resetProxyState(t)
	t.Setenv(proxyModeEnv, "direct")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	proxy, err := proxyFunc(nil)(req)
	if err != nil {
		t.Fatalf("proxy func error: %v", err)
	}
	if proxy != nil {
		t.Fatalf("expected direct proxy mode to return nil, got %v", proxy)
	}
}
