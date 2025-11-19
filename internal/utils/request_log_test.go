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

func TestLogStreamingRequestPayload_WritesToDedicatedFile(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv(requestLogEnvVar, logDir)
	resetStreamingLogDeduperForTest()

	payload := []byte("{\"task\":\"demo\"}")
	LogStreamingRequestPayload("req-123", payload)

	logPath := filepath.Join(logDir, requestLogRequestFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "req-123") {
		t.Fatalf("log missing request id: %s", content)
	}
	if !strings.Contains(content, string(payload)) {
		t.Fatalf("log missing payload: %s", content)
	}
}

func TestLogStreamingResponsePayload_WritesToDedicatedFile(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv(requestLogEnvVar, logDir)
	resetStreamingLogDeduperForTest()

	payload := []byte("{\"result\":\"demo\"}")
	LogStreamingResponsePayload("req-456", payload)

	logPath := filepath.Join(logDir, requestLogResponseFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "req-456") {
		t.Fatalf("log missing request id: %s", content)
	}
	if !strings.Contains(content, string(payload)) {
		t.Fatalf("log missing payload: %s", content)
	}
}

func TestLogStreamingPayload_DeduplicatesByFile(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv(requestLogEnvVar, logDir)
	resetStreamingLogDeduperForTest()

	payload := []byte("{\"task\":\"demo\"}")
	LogStreamingRequestPayload("req-dup", payload)
	LogStreamingRequestPayload("req-dup", payload)
	LogStreamingResponsePayload("req-dup", payload)
	LogStreamingResponsePayload("req-dup", payload)

	requestLogPath := filepath.Join(logDir, requestLogRequestFileName)
	responseLogPath := filepath.Join(logDir, requestLogResponseFileName)

	requestData, err := os.ReadFile(requestLogPath)
	if err != nil {
		t.Fatalf("failed to read request log file: %v", err)
	}
	responseData, err := os.ReadFile(responseLogPath)
	if err != nil {
		t.Fatalf("failed to read response log file: %v", err)
	}

	if strings.Count(string(requestData), "req-dup") != 1 {
		t.Fatalf("expected 1 request log entry for req-dup")
	}
	if strings.Count(string(responseData), "req-dup") != 1 {
		t.Fatalf("expected 1 response log entry for req-dup")
	}
}
