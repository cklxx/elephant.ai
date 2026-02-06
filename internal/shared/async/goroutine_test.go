package async

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubPanicLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *stubPanicLogger) Error(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func (l *stubPanicLogger) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.messages))
	copy(out, l.messages)
	return out
}

func TestGoRecoversPanic(t *testing.T) {
	logger := &stubPanicLogger{}
	done := make(chan struct{})

	Go(logger, "test", func() {
		defer close(done)
		panic("boom")
	})

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for goroutine")
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for {
		messages := logger.snapshot()
		for _, msg := range messages {
			if strings.Contains(msg, "goroutine panic [test]") {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected panic log, got %v", messages)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRecoverHandlesNilLogger(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	func() {
		defer Recover(nil, "nil-logger")
		panic("boom")
	}()
}
