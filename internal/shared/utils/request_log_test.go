package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetStreamingLogDeduperForTest() {
	streamingLogDeduper = sync.Map{}
}

func TestLogStreamingPayload_WritesSequentialEntries(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv(requestLogEnvVar, logDir)
	resetStreamingLogDeduperForTest()

	reqPayload := []byte("{\"task\":\"demo\"}")
	respPayload := []byte("{\"result\":\"demo\"}")
	LogStreamingRequestPayload("req-123", reqPayload)
	LogStreamingResponsePayload("req-123", respPayload)

	logPath := filepath.Join(logDir, requestLogFileName)
	entries := readRequestLogEntries(t, logPath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}

	if entries[0].entry.RequestID != "req-123" || entries[0].entry.EntryType != "request" {
		t.Fatalf("expected first entry to be request, got %+v", entries[0].entry)
	}
	if entries[1].entry.RequestID != "req-123" || entries[1].entry.EntryType != "response" {
		t.Fatalf("expected second entry to be response, got %+v", entries[1].entry)
	}
	if entries[0].rawPayload != string(reqPayload) {
		t.Fatalf("expected request payload to match, got %s", entries[0].rawPayload)
	}
	if entries[1].rawPayload != string(respPayload) {
		t.Fatalf("expected response payload to match, got %s", entries[1].rawPayload)
	}
}

func TestLogStreamingPayload_DeduplicatesByEntryType(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv(requestLogEnvVar, logDir)
	resetStreamingLogDeduperForTest()

	payload := []byte("{\"task\":\"demo\"}")
	LogStreamingRequestPayload("req-dup", payload)
	LogStreamingRequestPayload("req-dup", payload)
	LogStreamingResponsePayload("req-dup", payload)
	LogStreamingResponsePayload("req-dup", payload)

	logPath := filepath.Join(logDir, requestLogFileName)
	entries := readRequestLogEntries(t, logPath)

	if len(entries) != 2 {
		t.Fatalf("expected 2 deduped entries, got %d: %#v", len(entries), entries)
	}
	if entries[0].entry.EntryType != "request" || entries[1].entry.EntryType != "response" {
		t.Fatalf("expected request/response entries, got %+v / %+v", entries[0].entry, entries[1].entry)
	}
}

func TestLogStreamingErrorPayload_WritesErrorEntry(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv(requestLogEnvVar, logDir)
	resetStreamingLogDeduperForTest()

	LogStreamingErrorPayload("log-err-001:llm-1", LLMErrorLogDetails{
		Mode:       "complete",
		Provider:   "openai",
		Model:      "kimi-for-coding",
		Intent:     "unknown",
		Stage:      "retry_client",
		ErrorClass: "transient",
		Error:      "context deadline exceeded",
		LatencyMS:  60000,
	})

	logPath := filepath.Join(logDir, requestLogFileName)
	entries := readRequestLogEntries(t, logPath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 error entry, got %d", len(entries))
	}

	entry := entries[0].entry
	if entry.EntryType != "error" {
		t.Fatalf("expected entry_type=error, got %q", entry.EntryType)
	}
	if entry.LogID != "log-err-001" {
		t.Fatalf("expected derived log_id=log-err-001, got %q", entry.LogID)
	}
	if entry.ErrorClass != "transient" {
		t.Fatalf("expected error_class=transient, got %q", entry.ErrorClass)
	}
	if entry.Error == "" {
		t.Fatalf("expected non-empty error field")
	}
	if entry.Stage != "retry_client" {
		t.Fatalf("expected stage=retry_client, got %q", entry.Stage)
	}
	if entry.LatencyMS != 60000 {
		t.Fatalf("expected latency_ms=60000, got %d", entry.LatencyMS)
	}
	if entries[0].rawPayload == "" {
		t.Fatalf("expected non-empty raw payload line")
	}
}

// parsedEntry holds a decoded metadata entry and its raw payload line.
type parsedEntry struct {
	entry      requestLogEntry
	rawPayload string
}

func readRequestLogEntries(t *testing.T, path string) []parsedEntry {
	t.Helper()
	if !WaitForRequestLogQueueDrain(2 * time.Second) {
		t.Fatalf("timed out waiting for request log queue to drain")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	// Format: metadata JSON line, then raw payload line, repeating.
	var entries []parsedEntry
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry requestLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to decode log entry at line %d: %v", i, err)
		}
		// Next line is the raw payload (if present and not another JSON entry).
		rawPayload := ""
		if i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next != "" {
				var probe struct {
					EntryType string `json:"entry_type"`
				}
				if json.Unmarshal([]byte(next), &probe) != nil || probe.EntryType == "" {
					rawPayload = next
					i++ // consume the payload line
				}
			}
		}
		entries = append(entries, parsedEntry{entry: entry, rawPayload: rawPayload})
	}
	return entries
}
