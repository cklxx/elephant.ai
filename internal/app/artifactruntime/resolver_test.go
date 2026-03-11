package artifactruntime

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewAttachmentHTTPClient(t *testing.T) {
	client := NewAttachmentHTTPClient(3*time.Second, "artifactruntime-test")
	if client.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %v, want %v", client.Timeout, 3*time.Second)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", client.Transport)
	}
	if transport.TLSHandshakeTimeout != 3*time.Second {
		t.Fatalf("TLSHandshakeTimeout = %v, want %v", transport.TLSHandshakeTimeout, 3*time.Second)
	}
}

func TestResolveAttachmentBytes(t *testing.T) {
	t.Run("empty reference", func(t *testing.T) {
		_, _, err := ResolveAttachmentBytes(context.Background(), "   ", nil)
		if err == nil {
			t.Fatal("ResolveAttachmentBytes() error = nil, want error")
		}
	})

	t.Run("data uri", func(t *testing.T) {
		data, mediaType, err := ResolveAttachmentBytes(context.Background(), "data:text/plain;base64,aGVsbG8=", nil)
		if err != nil {
			t.Fatalf("ResolveAttachmentBytes() error = %v", err)
		}
		if string(data) != "hello" {
			t.Fatalf("data = %q, want hello", string(data))
		}
		if mediaType != "text/plain" {
			t.Fatalf("mediaType = %q, want text/plain", mediaType)
		}
	})
}
