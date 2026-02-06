package chromebridge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBridgeCallRequiresConnection(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0"})
	if _, err := bridge.Call(context.Background(), "bridge.ping", nil); !errors.Is(err, errNotConnected) {
		t.Fatalf("expected not-connected error, got %v", err)
	}
}

func TestBridgeHandshakeAndCall(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Token: "test-token", Timeout: 2 * time.Second})
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = bridge.Close(context.Background())
	})

	wsURL := "ws://" + bridge.Addr() + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := conn.WriteJSON(helloMessage{Type: "hello", Token: "test-token", Client: "chrome_extension", Version: 1}); err != nil {
		t.Fatalf("write hello returned error: %v", err)
	}
	var welcome welcomeMessage
	if err := conn.ReadJSON(&welcome); err != nil {
		t.Fatalf("read welcome returned error: %v", err)
	}
	if welcome.Type != "welcome" || welcome.Version != protocolVersion {
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
			if req.Method == "bridge.ping" {
				_ = conn.WriteJSON(rpcResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  json.RawMessage(`{"ok":true}`),
				})
			} else {
				_ = conn.WriteJSON(rpcResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &rpcError{
						Code:    -32601,
						Message: "method not found",
					},
				})
			}
		}
	}()

	raw, err := bridge.Call(context.Background(), "bridge.ping", nil)
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	var payload struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal result returned error: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok=true, got %#v", payload)
	}

	_ = conn.Close()
	<-done
}

func TestBridgeRejectsBadToken(t *testing.T) {
	bridge := New(Config{ListenAddr: "127.0.0.1:0", Token: "expected"})
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = bridge.Close(context.Background())
	})

	wsURL := "ws://" + bridge.Addr() + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial returned error: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(helloMessage{Type: "hello", Token: "wrong"}); err != nil {
		t.Fatalf("write hello returned error: %v", err)
	}
	var welcome welcomeMessage
	if err := conn.ReadJSON(&welcome); err == nil {
		t.Fatalf("expected handshake failure, got welcome: %+v", welcome)
	}
}
