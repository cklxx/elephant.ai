package httpclient

import (
	"testing"
	"time"
)

func TestNewDefaultsTimeout(t *testing.T) {
	client := New(0, nil)
	if client.Timeout != 30*time.Second {
		t.Fatalf("expected default timeout 30s, got %s", client.Timeout)
	}
	if client.Transport == nil {
		t.Fatal("expected transport to be set")
	}
}

func TestNewNoTimeoutKeepsZeroTimeout(t *testing.T) {
	client := NewNoTimeout(nil)
	if client.Timeout != 0 {
		t.Fatalf("expected timeout 0 for no-timeout client, got %s", client.Timeout)
	}
	if client.Transport == nil {
		t.Fatal("expected transport to be set")
	}
}
