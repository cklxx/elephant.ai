package app

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alex/internal/agent/ports"

	"github.com/stretchr/testify/require"
)

func TestHTTPAttachmentScannerCleanResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"verdict":"clean","details":"ok"}`))
	}))
	defer server.Close()

	scanner := NewHTTPAttachmentScanner(HTTPAttachmentScannerConfig{Endpoint: server.URL})
	require.NotNil(t, scanner)

	payload := []byte("payload")
	result, err := scanner.Scan(context.Background(), AttachmentScanRequest{
		SessionID:   "s",
		Placeholder: "[file]",
		MediaType:   "text/plain",
		Attachment:  ports.Attachment{Name: "file.txt"},
		Payload:     payload,
	})

	require.NoError(t, err)
	require.Equal(t, AttachmentScanVerdictClean, result.Verdict)
	require.Equal(t, "ok", result.Details)
}

func TestHTTPAttachmentScannerInfectedResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"verdict":"infected","details":"EICAR"}`))
	}))
	defer server.Close()

	scanner := NewHTTPAttachmentScanner(HTTPAttachmentScannerConfig{Endpoint: server.URL})

	result, err := scanner.Scan(context.Background(), AttachmentScanRequest{
		SessionID:  "s",
		Payload:    []byte("payload"),
		Attachment: ports.Attachment{Name: "virus.txt"},
	})

	require.NoError(t, err)
	require.Equal(t, AttachmentScanVerdictInfected, result.Verdict)
	require.Equal(t, "EICAR", result.Details)
}

func TestHTTPAttachmentScannerSignature(t *testing.T) {
	t.Parallel()
	var captured string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Get(attachmentSignatureHeader)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"verdict":"clean"}`))
	}))
	defer server.Close()

	scanner := NewHTTPAttachmentScanner(HTTPAttachmentScannerConfig{Endpoint: server.URL, Secret: "secret"})

	_, err := scanner.Scan(context.Background(), AttachmentScanRequest{
		SessionID:  "s",
		Payload:    []byte("payload"),
		Attachment: ports.Attachment{Name: "file"},
	})
	require.NoError(t, err)
	require.Contains(t, captured, "sha256=")
}

func TestHTTPAttachmentScannerRequestBody(t *testing.T) {
	t.Parallel()
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		received = string(data)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"verdict":"clean"}`))
	}))
	defer server.Close()

	scanner := NewHTTPAttachmentScanner(HTTPAttachmentScannerConfig{Endpoint: server.URL})

	payload := []byte("payload")
	_, err := scanner.Scan(context.Background(), AttachmentScanRequest{Payload: payload})
	require.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(payload)
	require.Contains(t, received, encoded)
}

func TestHTTPAttachmentScannerHandlesErrors(t *testing.T) {
	t.Parallel()
	scanner := NewHTTPAttachmentScanner(HTTPAttachmentScannerConfig{Endpoint: ""})
	require.Nil(t, scanner)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	scanner = NewHTTPAttachmentScanner(HTTPAttachmentScannerConfig{Endpoint: server.URL, Timeout: time.Millisecond})
	_, err := scanner.Scan(context.Background(), AttachmentScanRequest{Payload: []byte("payload")})
	require.Error(t, err)
}
