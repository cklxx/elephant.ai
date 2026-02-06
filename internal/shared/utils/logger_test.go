package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeLogLineLeavesContentUnchanged(t *testing.T) {
	lines := []string{
		"2024-10-10 [INFO] [ALEX] sample.go:10 - apiKey=sk-test12345678901234567890\n",
		"token Authorization: Bearer sk-secret-token-here",
		"random ghp_abcd1234efgh5678ijkl9012mnop3456 value",
	}

	for _, line := range lines {
		if got := sanitizeLogLine(line); got != line {
			t.Fatalf("expected log line to pass through unchanged, got %q", got)
		}
	}
}

func TestLoggerUsesOverriddenLogDirectory(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(logDirEnvVar, tempDir)
	ResetLoggerForTests(LogCategoryLatency)
	logger := NewLatencyLogger("test")
	logger.Info("hello world")
	if err := logger.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
	logPath := filepath.Join(tempDir, "alex-latency.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	contents := string(data)
	if !strings.Contains(contents, "hello world") {
		t.Fatalf("expected log line in overridden directory, got %s", contents)
	}
}
