package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogStreamingRequestPayload_WritesToDedicatedFile(t *testing.T) {
	t.Setenv(requestLogEnvVar, t.TempDir())

	payload := []byte("{\"task\":\"demo\"}")
	LogStreamingRequestPayload("req-123", payload)

	logDir, ok := os.LookupEnv(requestLogEnvVar)
	if !ok {
		t.Fatalf("expected %s to be set", requestLogEnvVar)
	}
	logPath := filepath.Join(logDir, requestLogFileName)
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
