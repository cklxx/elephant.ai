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
	writeTestLog(t, filepath.Join(logDir, "llm.jsonl"), "{\"timestamp\":\"2026-01-27T12:00:00Z\",\"request_id\":\"log-abc123:llm-1\",\"log_id\":\"log-abc123\",\"entry_type\":\"request\",\"body_bytes\":2,\"payload\":{}}\n")

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

func TestReadLogMatchesScansEntireFile(t *testing.T) {
	logDir := t.TempDir()
	// Write a file where matching entries appear after more than MaxBytes of non-matching data.
	var content strings.Builder
	for i := 0; i < 200; i++ {
		content.WriteString(strings.Repeat("x", 100) + " unrelated line\n")
	}
	// ~23KB of non-matching lines above; our match is at the end.
	content.WriteString("important log_id=log-deep-scan match\n")
	path := filepath.Join(logDir, "test.log")
	writeTestLog(t, path, content.String())

	// MaxBytes is only 1KB — much less than the file, but the match should still be found
	// because MaxBytes limits matched output size, not scan range.
	snippet := readLogMatches(path, "log-deep-scan", LogFetchOptions{
		MaxBytes:   1024,
		MaxEntries: 50,
	})

	if snippet.Error != "" {
		t.Fatalf("unexpected error: %s", snippet.Error)
	}
	if len(snippet.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(snippet.Entries))
	}
	if !strings.Contains(snippet.Entries[0], "log-deep-scan") {
		t.Fatalf("expected match, got: %s", snippet.Entries[0])
	}
}

func TestReadLogMatchesSkipsOversizedLines(t *testing.T) {
	logDir := t.TempDir()
	// Line 1: normal match
	// Line 2: oversized (exceeds MaxLineBytes), should be skipped
	// Line 3: normal match after the oversized line
	var content strings.Builder
	content.WriteString("first log_id=log-size match\n")
	content.WriteString(strings.Repeat("A", 2048) + " log_id=log-size oversized\n")
	content.WriteString("third log_id=log-size match\n")
	path := filepath.Join(logDir, "test.log")
	writeTestLog(t, path, content.String())

	snippet := readLogMatches(path, "log-size", LogFetchOptions{
		MaxBytes:     1 << 20,
		MaxEntries:   50,
		MaxLineBytes: 512, // Oversized line (2048+) exceeds this
	})

	if snippet.Error != "" {
		t.Fatalf("unexpected error: %s", snippet.Error)
	}
	// Should find lines 1 and 3, skipping the oversized line 2.
	if len(snippet.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(snippet.Entries))
	}
	if !strings.Contains(snippet.Entries[0], "first") {
		t.Fatalf("expected first match, got: %s", snippet.Entries[0])
	}
	if !strings.Contains(snippet.Entries[1], "third") {
		t.Fatalf("expected third match, got: %s", snippet.Entries[1])
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
