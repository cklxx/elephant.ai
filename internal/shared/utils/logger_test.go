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

func TestResolveLogLevel_DefaultINFO(t *testing.T) {
	t.Setenv(logLevelEnvVar, "")
	if got := resolveLogLevel(); got != INFO {
		t.Fatalf("expected INFO, got %v", got)
	}
}

func TestResolveLogLevel_Debug(t *testing.T) {
	t.Setenv(logLevelEnvVar, "DEBUG")
	if got := resolveLogLevel(); got != DEBUG {
		t.Fatalf("expected DEBUG, got %v", got)
	}
}

func TestResolveLogLevel_Warn(t *testing.T) {
	t.Setenv(logLevelEnvVar, "WARN")
	if got := resolveLogLevel(); got != WARN {
		t.Fatalf("expected WARN, got %v", got)
	}
}

func TestResolveLogLevel_Warning(t *testing.T) {
	t.Setenv(logLevelEnvVar, "warning")
	if got := resolveLogLevel(); got != WARN {
		t.Fatalf("expected WARN, got %v", got)
	}
}

func TestResolveLogLevel_Error(t *testing.T) {
	t.Setenv(logLevelEnvVar, "ERROR")
	if got := resolveLogLevel(); got != ERROR {
		t.Fatalf("expected ERROR, got %v", got)
	}
}

func TestLoggerDefaultsToINFO_SuppressesDebug(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(logDirEnvVar, tempDir)
	t.Setenv(logLevelEnvVar, "")
	ResetLoggerForTests(LogCategoryService)
	logger := NewComponentLogger("test")
	logger.Debug("should-not-appear")
	logger.Info("should-appear")
	if err := logger.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
	logPath := filepath.Join(tempDir, "alex-service.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	contents := string(data)
	if strings.Contains(contents, "should-not-appear") {
		t.Fatalf("expected DEBUG messages to be suppressed at default INFO level")
	}
	if !strings.Contains(contents, "should-appear") {
		t.Fatalf("expected INFO messages to appear")
	}
}

func TestLoggerDebugLevelShowsDebug(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(logDirEnvVar, tempDir)
	t.Setenv(logLevelEnvVar, "DEBUG")
	ResetLoggerForTests(LogCategoryService)
	logger := NewComponentLogger("test")
	logger.Debug("debug-visible")
	if err := logger.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
	logPath := filepath.Join(tempDir, "alex-service.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "debug-visible") {
		t.Fatalf("expected DEBUG messages to appear with ALEX_LOG_LEVEL=DEBUG")
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
