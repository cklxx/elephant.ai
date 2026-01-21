package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jsonrpc "alex/internal/mcp"

	"github.com/stretchr/testify/require"
)

type testSSEBroker struct {
	mu      sync.Mutex
	writer  http.ResponseWriter
	flusher http.Flusher
	ready   chan struct{}
}

func newTestSSEBroker() *testSSEBroker {
	return &testSSEBroker{ready: make(chan struct{})}
}

func (b *testSSEBroker) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	b.mu.Lock()
	b.writer = w
	b.flusher = flusher
	select {
	case <-b.ready:
	default:
		close(b.ready)
	}
	b.mu.Unlock()

	<-r.Context().Done()
}

func (b *testSSEBroker) handleRPC(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read failed", http.StatusBadRequest)
		return
	}
	req, resp, err := ParseRPCPayload(body)
	if err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	if resp != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req == nil {
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}
	response := jsonrpc.NewResponse(req.ID, map[string]any{"ok": true})
	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "marshal failed", http.StatusInternalServerError)
		return
	}
	b.send(payload)
	w.WriteHeader(http.StatusNoContent)
}

func (b *testSSEBroker) waitReady(t *testing.T) {
	t.Helper()
	select {
	case <-b.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE stream did not connect")
	}
}

func (b *testSSEBroker) send(payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.writer == nil || b.flusher == nil {
		return
	}
	fmt.Fprintf(b.writer, "data: %s\n\n", payload)
	b.flusher.Flush()
}

func TestSSEClientCall(t *testing.T) {
	broker := newTestSSEBroker()
	mux := http.NewServeMux()
	mux.HandleFunc("/acp/sse", broker.handleSSE)
	mux.HandleFunc("/acp/rpc", broker.handleRPC)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("network listen not permitted: %v", err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: mux},
	}
	server.Start()
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := Dial(ctx, server.URL, time.Second, nil)
	require.NoError(t, err)
	client.Start(ctx, nil)

	broker.waitReady(t)

	resp, err := client.Call(ctx, "initialize", map[string]any{"protocolVersion": 1})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)
	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, result["ok"])
}

func TestNormalizeACPAddr(t *testing.T) {
	out, err := normalizeACPAddr("127.0.0.1:9000")
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:9000", out)

	out, err = normalizeACPAddr("https://example.com")
	require.NoError(t, err)
	require.Equal(t, "https://example.com", out)
}

func TestIsRetryableError(t *testing.T) {
	require.True(t, IsRetryableError(context.DeadlineExceeded))
	require.True(t, IsRetryableError(&RPCStatusError{StatusCode: 503}))
	require.False(t, IsRetryableError(&RPCStatusError{StatusCode: 404}))
	require.True(t, IsRetryableError(&url.Error{Op: "post", URL: "http://127.0.0.1:9000", Err: io.EOF}))
}
