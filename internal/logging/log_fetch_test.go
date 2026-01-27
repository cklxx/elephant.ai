package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestLog(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func TestFetchLogBundleCollectsMatches(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("ALEX_LOG_DIR", logDir)
	t.Setenv("ALEX_REQUEST_LOG_DIR", logDir)

	logID := "log-abc123"

	writeTestLog(t, filepath.Join(logDir, "alex-service.log"), "line log_id=log-abc123 service\nother line\n")
	writeTestLog(t, filepath.Join(logDir, "alex-llm.log"), "[req:log-abc123:llm-1] request\n")
	writeTestLog(t, filepath.Join(logDir, "alex-latency.log"), "latency log_id=log-abc123 ms=12\n")
	writeTestLog(t, filepath.Join(logDir, "streaming.log"), "2026-01-27 [req:log-abc123:llm-1] [request] body_bytes=2\n{}\n\n")

	bundle := FetchLogBundle(logID, LogFetchOptions{MaxBytes: 1024, MaxEntries: 20})

	if bundle.LogID != logID {
		t.Fatalf("expected log id %s, got %s", logID, bundle.LogID)
	}
	if len(bundle.Service.Entries) != 1 || !strings.Contains(bundle.Service.Entries[0], logID) {
		t.Fatalf("expected service log match, got %#v", bundle.Service)
	}
	if len(bundle.LLM.Entries) != 1 || !strings.Contains(bundle.LLM.Entries[0], logID) {
		t.Fatalf("expected llm log match, got %#v", bundle.LLM)
	}
	if len(bundle.Latency.Entries) != 1 || !strings.Contains(bundle.Latency.Entries[0], logID) {
		t.Fatalf("expected latency log match, got %#v", bundle.Latency)
	}
	if len(bundle.Requests.Entries) != 1 || !strings.Contains(bundle.Requests.Entries[0], logID) {
		t.Fatalf("expected request log match, got %#v", bundle.Requests)
	}
}

func TestFetchLogBundleFlagsTruncation(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("ALEX_LOG_DIR", logDir)

	logID := "log-truncate"
	content := strings.Repeat("x", 2048) + " log_id=log-truncate end\n"
	writeTestLog(t, filepath.Join(logDir, "alex-service.log"), content)

	bundle := FetchLogBundle(logID, LogFetchOptions{MaxBytes: 128, MaxEntries: 10})

	if !bundle.Service.Truncated {
		t.Fatalf("expected truncation to be true")
	}
	if len(bundle.Service.Entries) == 0 {
		t.Fatalf("expected at least one entry in truncated logs")
	}
}
