package main

import (
	"context"
	"sync"
	"testing"

	id "alex/internal/utils/id"
)

func resetCLIBaseContextForTest() {
	cliBaseOnce = sync.Once{}
	cliBaseCtx = nil
	cliBaseCancel = nil
}

func TestCLIBaseContextProvidesLogID(t *testing.T) {
	resetCLIBaseContextForTest()
	t.Cleanup(resetCLIBaseContextForTest)

	ctx := cliBaseContext()
	if id.LogIDFromContext(ctx) == "" {
		t.Fatalf("expected log id to be set")
	}
}

func TestCLIBaseContextCancellation(t *testing.T) {
	resetCLIBaseContextForTest()
	t.Cleanup(resetCLIBaseContextForTest)

	ctx := cliBaseContext()
	ctx2 := cliBaseContext()
	select {
	case <-ctx.Done():
		t.Fatalf("expected base context to be active")
	default:
	}

	cancelCLIBaseContext()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("expected cancellation error to be set")
		}
	default:
		t.Fatalf("expected base context to be canceled")
	}

	select {
	case <-ctx2.Done():
		if ctx2.Err() != context.Canceled {
			t.Fatalf("expected cancellation error to be set for derived context")
		}
	default:
		t.Fatalf("expected derived context to be canceled")
	}
}
