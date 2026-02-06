package chromebridge

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"

	"github.com/gorilla/websocket"
)

func startTestExtension(t *testing.T, bridge *Bridge, token string, handler func(rpcRequest) rpcResponse) (*websocket.Conn, <-chan struct{}) {
	t.Helper()

	wsURL := "ws://" + bridge.Addr() + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial returned error: %v", err)
	}

	if err := conn.WriteJSON(helloMessage{Type: "hello", Token: token, Client: "chrome_extension", Version: 1}); err != nil {
		_ = conn.Close()
		t.Fatalf("write hello returned error: %v", err)
	}
	var welcome welcomeMessage
	if err := conn.ReadJSON(&welcome); err != nil {
		_ = conn.Close()
		t.Fatalf("read welcome returned error: %v", err)
	}
	if welcome.Type != "welcome" || welcome.Version != protocolVersion {
		_ = conn.Close()
		t.Fatalf("unexpected welcome message: %+v", welcome)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var req rpcRequest
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if req.JSONRPC != "2.0" || req.ID == "" {
				continue
			}
			resp := handler(req)
			resp.JSONRPC = "2.0"
			resp.ID = req.ID
			_ = conn.WriteJSON(resp)
		}
	}()

	return conn, done
}

func TestBrowserSessionStatusNotConnected(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Timeout: 1 * time.Second})
	tool := NewBrowserSessionStatus(bridge)

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected tool result")
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got %v", result.Error)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal content returned error: %v", err)
	}
	bridgeObj, _ := payload["bridge"].(map[string]any)
	if connected, _ := bridgeObj["connected"].(bool); connected {
		t.Fatalf("expected connected=false, got %#v", bridgeObj["connected"])
	}
	if listen, _ := bridgeObj["listen_addr"].(string); listen == "" {
		t.Fatalf("expected listen_addr to be set, got %#v", bridgeObj["listen_addr"])
	}
}

func TestBrowserSessionStatusConnectedTabsLimit(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Token: "t", Timeout: 2 * time.Second})
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close(context.Background()) })

	conn, done := startTestExtension(t, bridge, "t", func(req rpcRequest) rpcResponse {
		switch req.Method {
		case "tabs.list":
			return rpcResponse{
				Result: json.RawMessage(`[
  {"tabId": 101, "windowId": 1, "url": "https://example.com", "title": "Example", "active": true},
  {"tabId": 102, "windowId": 1, "url": "https://x.com", "title": "X", "active": false}
]`),
			}
		default:
			return rpcResponse{Error: &rpcError{Code: -32601, Message: "method not found"}}
		}
	})
	t.Cleanup(func() {
		_ = conn.Close()
		<-done
	})

	if err := bridge.WaitForConnected(context.Background(), time.Second); err != nil {
		t.Fatalf("WaitForConnected returned error: %v", err)
	}

	tool := NewBrowserSessionStatus(bridge)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"max_tabs": float64(1),
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success result, got %#v", result)
	}

	var payload struct {
		Tabs []chromeTab `json:"tabs"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal content returned error: %v", err)
	}
	if len(payload.Tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(payload.Tabs))
	}
	if payload.Tabs[0].TabID != 101 {
		t.Fatalf("unexpected tabId: %+v", payload.Tabs[0])
	}
}

func TestBrowserCookiesRequiresConnection(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Timeout: 1 * time.Second})
	tool := NewBrowserCookies(bridge)

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-3",
		Arguments: map[string]any{
			"domain": "xiaohongshu.com",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected tool result")
	}
	if result.Error == nil {
		t.Fatalf("expected tool error")
	}
}

func TestBrowserCookiesHeader(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Token: "t", Timeout: 2 * time.Second})
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close(context.Background()) })

	conn, done := startTestExtension(t, bridge, "t", func(req rpcRequest) rpcResponse {
		switch req.Method {
		case "cookies.toHeader":
			return rpcResponse{Result: json.RawMessage(`"a=1; b=2"`)}
		default:
			return rpcResponse{Error: &rpcError{Code: -32601, Message: "method not found"}}
		}
	})
	t.Cleanup(func() {
		_ = conn.Close()
		<-done
	})

	tool := NewBrowserCookies(bridge)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-4",
		Arguments: map[string]any{
			"domain": "xiaohongshu.com",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success, got %#v", result)
	}

	var payload struct {
		CookieHeader string `json:"cookie_header"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal content returned error: %v", err)
	}
	if payload.CookieHeader != "a=1; b=2" {
		t.Fatalf("unexpected cookie_header: %q", payload.CookieHeader)
	}
}

func TestBrowserCookiesJSON(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Token: "t", Timeout: 2 * time.Second})
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close(context.Background()) })

	conn, done := startTestExtension(t, bridge, "t", func(req rpcRequest) rpcResponse {
		switch req.Method {
		case "cookies.getAll":
			return rpcResponse{
				Result: json.RawMessage(`[
  {"name": "a", "value": "1", "domain": ".xiaohongshu.com", "path": "/", "secure": true, "httpOnly": true},
  {"name": "b", "value": "2", "domain": "www.xiaohongshu.com", "path": "/", "secure": false, "httpOnly": false}
]`),
			}
		case "cookies.toHeader":
			return rpcResponse{Result: json.RawMessage(`"a=1; b=2"`)}
		default:
			return rpcResponse{Error: &rpcError{Code: -32601, Message: "method not found"}}
		}
	})
	t.Cleanup(func() {
		_ = conn.Close()
		<-done
	})

	tool := NewBrowserCookies(bridge)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-5",
		Arguments: map[string]any{
			"domain": "xiaohongshu.com",
			"format": "json",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success, got %#v", result)
	}

	var payload struct {
		Cookies []chromeCookie `json:"cookies"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal content returned error: %v", err)
	}
	if len(payload.Cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(payload.Cookies))
	}
}

func TestBrowserStorageLocal(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Token: "t", Timeout: 2 * time.Second})
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close(context.Background()) })

	conn, done := startTestExtension(t, bridge, "t", func(req rpcRequest) rpcResponse {
		switch req.Method {
		case "storage.getLocal":
			return rpcResponse{Result: json.RawMessage(`{"token":"abc","missing":null}`)}
		default:
			return rpcResponse{Error: &rpcError{Code: -32601, Message: "method not found"}}
		}
	})
	t.Cleanup(func() {
		_ = conn.Close()
		<-done
	})

	tool := NewBrowserStorageLocal(bridge)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-6",
		Arguments: map[string]any{
			"tab_id": float64(123),
			"keys":   []any{"token", "missing"},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success, got %#v", result)
	}
}
