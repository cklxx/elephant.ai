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

	if entries[0].RequestID != "req-123" || entries[0].EntryType != "request" {
		t.Fatalf("expected first entry to be request, got %+v", entries[0])
	}
	if entries[1].RequestID != "req-123" || entries[1].EntryType != "response" {
		t.Fatalf("expected second entry to be response, got %+v", entries[1])
	}
	if string(entries[0].Payload) != string(reqPayload) {
		t.Fatalf("expected request payload to match, got %s", string(entries[0].Payload))
	}
	if string(entries[1].Payload) != string(respPayload) {
		t.Fatalf("expected response payload to match, got %s", string(entries[1].Payload))
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
	if entries[0].EntryType != "request" || entries[1].EntryType != "response" {
		t.Fatalf("expected request/response entries, got %#v", entries)
	}
}

func readRequestLogEntries(t *testing.T, path string) []requestLogEntry {
	t.Helper()
	if !WaitForRequestLogQueueDrain(2 * time.Second) {
		t.Fatalf("timed out waiting for request log queue to drain")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	entries := make([]requestLogEntry, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry requestLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to decode log entry: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}
