package utils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "req-123") {
		t.Fatalf("log missing request id: %s", content)
	}
	if !strings.Contains(content, string(reqPayload)) {
		t.Fatalf("log missing raw request payload: %s", content)
	}
	if !strings.Contains(content, string(respPayload)) {
		t.Fatalf("log missing raw response payload: %s", content)
	}
	requestIndex := strings.Index(content, string(reqPayload))
	responseIndex := strings.Index(content, string(respPayload))
	if requestIndex == -1 || responseIndex == -1 {
		t.Fatalf("expected request and response payloads in log: %s", content)
	}
	if requestIndex >= responseIndex {
		t.Fatalf("expected request payload to be logged before response: %s", content)
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
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	content := string(data)
	if strings.Count(content, "[request]") != 1 {
		t.Fatalf("expected 1 request log entry for req-dup: %s", content)
	}
	if strings.Count(content, "[response]") != 1 {
		t.Fatalf("expected 1 response log entry for req-dup: %s", content)
	}
}
