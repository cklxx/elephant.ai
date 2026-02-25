package acp

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	jsonrpc "alex/internal/infra/mcp"
)

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestPendingResponseHelpers(t *testing.T) {
	var pendingMu sync.Mutex
	pending := make(map[string]chan *jsonrpc.Response)

	ch := registerPendingResponse(&pendingMu, pending, "1")
	if ch == nil {
		t.Fatal("expected pending channel")
	}

	pendingMu.Lock()
	stored := pending["1"]
	pendingMu.Unlock()
	if stored != ch {
		t.Fatal("expected registered channel in pending map")
	}

	if got, ok := popPendingResponse(&pendingMu, pending, "missing"); ok || got != nil {
		t.Fatal("expected missing key to return nil,false")
	}

	popped, ok := popPendingResponse(&pendingMu, pending, "1")
	if !ok || popped != ch {
		t.Fatal("expected to pop the registered channel")
	}

	pendingMu.Lock()
	remaining := len(pending)
	pendingMu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected pending map to be empty, got %d", remaining)
	}

	registerPendingResponse(&pendingMu, pending, "2")
	deletePendingResponse(&pendingMu, pending, "2")
	pendingMu.Lock()
	remaining = len(pending)
	pendingMu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected pending map to be empty after delete, got %d", remaining)
	}
}

func TestRPCConnCall_CleansPendingOnSendError(t *testing.T) {
	conn := NewRPCConn(strings.NewReader(""), failingWriter{})

	_, err := conn.Call(context.Background(), "test.method", nil)
	if err == nil {
		t.Fatal("expected call error")
	}

	conn.pendingMu.Lock()
	remaining := len(conn.pending)
	conn.pendingMu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected pending map to be empty, got %d", remaining)
	}
}

func TestRPCConnCall_CleansPendingOnContextDone(t *testing.T) {
	conn := NewRPCConn(strings.NewReader(""), &bytes.Buffer{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := conn.Call(ctx, "test.method", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}

	conn.pendingMu.Lock()
	remaining := len(conn.pending)
	conn.pendingMu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected pending map to be empty, got %d", remaining)
	}
}
