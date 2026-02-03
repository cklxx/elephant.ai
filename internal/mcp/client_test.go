package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"testing"
	"time"
)

func TestClient_handleLine_RoutesNumericResponseID(t *testing.T) {
	c := NewClient("test", nil)

	ch := make(chan *Response, 1)
	c.pendingCalls["42"] = ch

	c.handleLine([]byte(`{"jsonrpc":"2.0","id":42,"result":{"ok":true}}`))

	select {
	case resp := <-ch:
		if resp == nil {
			t.Fatalf("expected response, got nil")
		}
		if resp.IsError() {
			t.Fatalf("expected non-error response")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for routed response")
	}
}

func TestClient_handleLine_RoutesStringResponseID(t *testing.T) {
	c := NewClient("test", nil)

	ch := make(chan *Response, 1)
	c.pendingCalls["req-1"] = ch

	c.handleLine([]byte(`{"jsonrpc":"2.0","id":"req-1","result":"ok"}`))

	select {
	case resp := <-ch:
		if resp == nil {
			t.Fatalf("expected response, got nil")
		}
		if got, ok := resp.Result.(string); !ok || got != "ok" {
			t.Fatalf("unexpected result: %#v", resp.Result)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for routed response")
	}
}

func TestClient_handleLine_DispatchesNotifications(t *testing.T) {
	c := NewClient("test", nil)

	received := make(chan string, 1)
	c.SetNotificationHandler(func(method string, params map[string]any) {
		received <- method
	})

	c.handleLine([]byte(`{"jsonrpc":"2.0","method":"codex/event","params":{"msg":{"type":"token_count"}}}`))

	select {
	case method := <-received:
		if method != "codex/event" {
			t.Fatalf("unexpected method: %q", method)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for notification")
	}
}

func TestClient_initialize_IncludesCapabilitiesAndNormalizesResponseID(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	pm := &ProcessManager{
		stdin:   stdinW,
		stdout:  stdoutR,
		running: true,
		stopChan: make(chan struct{}),
	}
	c := NewClient("test", pm)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start client read loop.
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.readLoop()
	}()

	serverErr := make(chan error, 1)
	go func() {
		br := bufio.NewReader(stdinR)

		// 1) initialize request
		line, err := br.ReadBytes('\n')
		if err != nil {
			serverErr <- err
			return
		}

		var req map[string]any
		if err := json.Unmarshal(line, &req); err != nil {
			serverErr <- err
			return
		}
		params, _ := req["params"].(map[string]any)
		if params == nil {
			serverErr <- strconv.ErrSyntax
			return
		}
		if _, ok := params["capabilities"]; !ok {
			serverErr <- strconv.ErrSyntax
			return
		}

		idRaw := req["id"]
		idStr, ok := idRaw.(string)
		if !ok {
			serverErr <- strconv.ErrSyntax
			return
		}
		idNum, err := strconv.Atoi(idStr)
		if err != nil {
			serverErr <- err
			return
		}

		resp := map[string]any{
			"jsonrpc": JSONRPCVersion,
			"id":      idNum, // intentionally numeric to validate normalization
			"result": map[string]any{
				"protocolVersion": MCPProtocolVersion,
				"serverInfo": map[string]any{
					"name":    "fake",
					"version": "0.0.1",
				},
				"capabilities": map[string]any{},
			},
		}
		b, err := json.Marshal(resp)
		if err != nil {
			serverErr <- err
			return
		}
		if _, err := stdoutW.Write(append(b, '\n')); err != nil {
			serverErr <- err
			return
		}

		// 2) notifications/initialized notification (best-effort: just consume it)
		if _, err := br.ReadBytes('\n'); err != nil {
			serverErr <- err
			return
		}

		_ = stdoutW.Close()
		_ = stdinR.Close()
		serverErr <- nil
	}()

	if err := c.initialize(ctx); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	if err := <-serverErr; err != nil {
		t.Fatalf("server failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("read loop did not exit")
	}
}
