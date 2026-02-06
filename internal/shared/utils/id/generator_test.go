package id

import (
	"strings"
	"testing"
)

func TestNewRequestIDWithLogID(t *testing.T) {
	requestID := NewRequestIDWithLogID("log-123")
	if !strings.HasPrefix(requestID, "log-123:llm-") {
		t.Fatalf("expected request id to embed log id, got %q", requestID)
	}

	fallback := NewRequestIDWithLogID(" ")
	if !strings.HasPrefix(fallback, "llm-") {
		t.Fatalf("expected request id to fall back to llm prefix, got %q", fallback)
	}
}
