package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"net/http/httptest"
)

func TestSSEStreamFlushesHeaders(t *testing.T) {
	transport := newSSETransport("test-client", nil)
	if transport == nil {
		t.Fatal("expected transport")
	}

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.Stream(ctx, rec)
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rec.Body.Len() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if rec.Body.Len() == 0 {
		cancel()
		<-errCh
		t.Fatal("expected initial SSE comment to be flushed")
	}

	if !strings.Contains(rec.Body.String(), ": connected") {
		cancel()
		<-errCh
		t.Fatalf("expected initial SSE comment, got: %q", rec.Body.String())
	}

	cancel()
	<-errCh
}
