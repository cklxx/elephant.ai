package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	agentports "alex/internal/agent/ports"

	"github.com/stretchr/testify/require"
)

func TestHTTPAttachmentExporterRetriesUntilSuccess(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	exporter := NewHTTPAttachmentExporterWithConfig(HTTPAttachmentExporterConfig{
		Endpoint:    server.URL,
		MaxAttempts: 3,
		Backoff:     10 * time.Millisecond,
		HTTPClient:  server.Client(),
	})

	result := exporter.ExportSession(context.Background(), "session", map[string]agentports.Attachment{
		"report.txt": {Name: "report.txt"},
	})

	require.GreaterOrEqual(t, atomic.LoadInt32(&attempts), int32(3))
	require.True(t, result.Exported, "expected exporter to eventually succeed")
	require.Equal(t, 3, result.Attempts)
}

func TestHTTPAttachmentExporterSignsPayload(t *testing.T) {
	t.Parallel()

	secret := "top-secret"
	signatureCh := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		expected := computeHMACHex(body, []byte(secret))
		signature := r.Header.Get(attachmentSignatureHeader)
		require.Equal(t, "sha256="+expected, signature)
		signatureCh <- signature
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	exporter := NewHTTPAttachmentExporterWithConfig(HTTPAttachmentExporterConfig{
		Endpoint:   server.URL,
		Secret:     secret,
		HTTPClient: server.Client(),
	})

	result := exporter.ExportSession(context.Background(), "session", map[string]agentports.Attachment{
		"analysis.png": {Name: "analysis.png"},
	})

	select {
	case sig := <-signatureCh:
		require.NotEmpty(t, sig)
		// The handler already validated the signature; just ensure it was sent.
	case <-time.After(time.Second):
		t.Fatalf("signature not received")
	}
	require.True(t, result.Exported, "expected signed export to succeed")
}

func TestHTTPAttachmentExporterCapturesAttachmentUpdates(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"attachments":{"report.txt":{"uri":"https://cdn.example/report.txt"}}}`)
	}))
	defer server.Close()
	exporter := NewHTTPAttachmentExporterWithConfig(HTTPAttachmentExporterConfig{
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
	})
	result := exporter.ExportSession(context.Background(), "session", map[string]agentports.Attachment{
		"report.txt": {Name: "report.txt"},
	})
	require.True(t, result.Exported)
	if result.AttachmentUpdates == nil {
		t.Fatalf("expected attachment updates to be captured")
	}
	if uri := result.AttachmentUpdates["report.txt"].URI; uri != "https://cdn.example/report.txt" {
		t.Fatalf("expected uri to be propagated, got %s", uri)
	}
}

func computeHMACHex(body []byte, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
