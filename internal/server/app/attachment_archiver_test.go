package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/utils"

	"github.com/stretchr/testify/require"
)

func TestSandboxAttachmentArchiverDecodeAttachmentPayload_Remote(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=utf-8")
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	archiver := &sandboxAttachmentArchiver{httpClient: server.Client(), hostFilter: newHostFilter(SandboxAttachmentArchiverConfig{})}

	attachment := ports.Attachment{URI: server.URL + "/file.png"}
	payload, mediaType, err := archiver.decodeAttachmentPayload(context.Background(), attachment)

	require.NoError(t, err)
	require.Equal(t, []byte("payload"), payload)
	require.Equal(t, "image/png", mediaType)
}

func TestSandboxAttachmentArchiverDecodeAttachmentPayload_RemoteTooLarge(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := bytes.Repeat([]byte{'a'}, maxRemoteAttachmentBytes+1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	archiver := &sandboxAttachmentArchiver{httpClient: server.Client(), hostFilter: newHostFilter(SandboxAttachmentArchiverConfig{})}

	_, _, err := archiver.decodeAttachmentPayload(context.Background(), ports.Attachment{URI: server.URL})
	require.Error(t, err)
}

func TestSandboxAttachmentArchiverDecodeAttachmentPayload_RemoteHostFilters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	allowedArchiver := &sandboxAttachmentArchiver{
		httpClient: server.Client(),
		hostFilter: newHostFilter(SandboxAttachmentArchiverConfig{AllowedRemoteHosts: []string{"127.0.0.1"}}),
	}

	_, _, err := allowedArchiver.decodeAttachmentPayload(context.Background(), ports.Attachment{URI: server.URL})
	require.NoError(t, err)

	blockedArchiver := &sandboxAttachmentArchiver{
		httpClient: server.Client(),
		hostFilter: newHostFilter(SandboxAttachmentArchiverConfig{BlockedRemoteHosts: []string{"127.0.0.1"}}),
	}

	_, _, err = blockedArchiver.decodeAttachmentPayload(context.Background(), ports.Attachment{URI: server.URL})
	require.Error(t, err)
}

func TestAttachmentCacheReserveAndDeduplicate(t *testing.T) {
	cache := &attachmentCache{}

	first := cache.Reserve("report.txt")
	if first != "report.txt" {
		t.Fatalf("expected first reservation to keep name, got %s", first)
	}

	second := cache.Reserve("report.txt")
	if second == first {
		t.Fatalf("expected second reservation to gain suffix, got %s", second)
	}

	cache.Release(second)
	third := cache.Reserve("report.txt")
	if third == first {
		t.Fatalf("expected released slot to still add suffix to avoid collision, got %s", third)
	}

	cache.Remember("hash-one", first)
	if stored, ok := cache.HasDigest("hash-one"); !ok || stored != first {
		t.Fatalf("expected digest lookup to return %s, got %s (ok=%v)", first, stored, ok)
	}
}

func TestSandboxAttachmentArchiverPreparePayloadScannerBlocks(t *testing.T) {
	t.Parallel()
	scanner := &stubAttachmentScanner{result: AttachmentScanResult{Verdict: AttachmentScanVerdictInfected, Details: "virus"}}
	archiver := &sandboxAttachmentArchiver{scanner: scanner, logger: utils.NewComponentLogger("test")}
	att := ports.Attachment{Data: base64.StdEncoding.EncodeToString([]byte("payload")), MediaType: "text/plain"}
	payload, mediaType, ok := archiver.preparePayload(context.Background(), "session", "task", "report.txt", att)
	require.False(t, ok)
	require.Nil(t, payload)
	require.Equal(t, "", mediaType)
	require.Len(t, scanner.requests, 1)
}

func TestSandboxAttachmentArchiverPreparePayloadScannerErrors(t *testing.T) {
	t.Parallel()
	scanner := &stubAttachmentScanner{err: fmt.Errorf("scanner down")}
	archiver := &sandboxAttachmentArchiver{scanner: scanner}
	att := ports.Attachment{Data: base64.StdEncoding.EncodeToString([]byte("payload")), MediaType: "text/plain"}
	payload, mediaType, ok := archiver.preparePayload(context.Background(), "session", "task", "report.txt", att)
	require.True(t, ok)
	require.Equal(t, []byte("payload"), payload)
	require.Equal(t, "text/plain", mediaType)
	require.Len(t, scanner.requests, 1)
}

func TestSandboxAttachmentArchiverReportsScanVerdicts(t *testing.T) {
	reporter := &stubScanReporter{}
	scanner := &stubAttachmentScanner{result: AttachmentScanResult{Verdict: AttachmentScanVerdictInfected, Details: "virus"}}
	archiver := &sandboxAttachmentArchiver{scanner: scanner, scanReporter: reporter, logger: utils.NewComponentLogger("test")}
	att := ports.Attachment{Name: "blocked.png", Data: base64.StdEncoding.EncodeToString([]byte("payload")), MediaType: "image/png"}
	_, _, ok := archiver.preparePayload(context.Background(), "session", "task", "blocked.png", att)
	require.False(t, ok)
	require.Len(t, reporter.calls, 1)
	require.Equal(t, "task", reporter.calls[0].taskID)
	require.Equal(t, "blocked.png", reporter.calls[0].placeholder)
	require.Equal(t, AttachmentScanVerdictInfected, reporter.calls[0].result.Verdict)
}

type stubAttachmentScanner struct {
	result   AttachmentScanResult
	err      error
	requests []AttachmentScanRequest
}

func (s *stubAttachmentScanner) Scan(ctx context.Context, req AttachmentScanRequest) (AttachmentScanResult, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return AttachmentScanResult{}, s.err
	}
	return s.result, nil
}

type scanReport struct {
	sessionID   string
	taskID      string
	placeholder string
	attachment  ports.Attachment
	result      AttachmentScanResult
}

type stubScanReporter struct {
	calls []scanReport
}

func (s *stubScanReporter) ReportAttachmentScan(sessionID, taskID, placeholder string, attachment ports.Attachment, result AttachmentScanResult) {
	s.calls = append(s.calls, scanReport{
		sessionID:   sessionID,
		taskID:      taskID,
		placeholder: placeholder,
		attachment:  attachment,
		result:      result,
	})
}
