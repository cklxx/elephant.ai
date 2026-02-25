package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type sseRecorder struct {
	mu      sync.Mutex
	header  http.Header
	body    bytes.Buffer
	wroteCh chan struct{}
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{
		header:  make(http.Header),
		wroteCh: make(chan struct{}),
	}
}

func (r *sseRecorder) Header() http.Header {
	return r.header
}

func (r *sseRecorder) WriteHeader(statusCode int) {
	_ = statusCode
}

func (r *sseRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, err := r.body.Write(p)
	if n > 0 {
		select {
		case <-r.wroteCh:
		default:
			close(r.wroteCh)
		}
	}
	return n, err
}

func (r *sseRecorder) Flush() {}

func (r *sseRecorder) BodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func TestSSEStreamFlushesHeaders(t *testing.T) {
	transport := newSSETransport("test-client", nil)
	if transport == nil {
		t.Fatal("expected transport")
	}

	rec := newSSERecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.Stream(ctx, rec)
	}()

	select {
	case <-rec.wroteCh:
	case <-time.After(500 * time.Millisecond):
		cancel()
		<-errCh
		t.Fatal("expected initial SSE comment to be flushed")
	}

	if !strings.Contains(rec.BodyString(), ": connected") {
		cancel()
		<-errCh
		t.Fatalf("expected initial SSE comment, got: %q", rec.BodyString())
	}

	cancel()
	<-errCh
}
