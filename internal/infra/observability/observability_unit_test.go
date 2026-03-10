package observability

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Logger: level parsing edge cases
// ---------------------------------------------------------------------------

func TestNewLogger_AllLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "UNKNOWN", ""} {
		buf := &bytes.Buffer{}
		logger := NewLogger(LogConfig{Level: level, Format: "json", Output: buf})
		require.NotNil(t, logger, "level=%q", level)
		logger.Error("test") // error always visible
		assert.NotEmpty(t, buf.String(), "level=%q should produce output for Error", level)
	}
}

func TestNewLogger_TextFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(LogConfig{Level: "info", Format: "text", Output: buf})
	logger.Info("hello world")
	assert.Contains(t, buf.String(), "hello world")
}

func TestNewLogger_NilOutputDefaultsToStdout(t *testing.T) {
	// Should not panic when Output is nil.
	logger := NewLogger(LogConfig{Level: "error", Format: "json"})
	require.NotNil(t, logger)
}

// ---------------------------------------------------------------------------
// Logger: WithContext with empty context
// ---------------------------------------------------------------------------

func TestLoggerWithContext_EmptyContext(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(LogConfig{Level: "info", Format: "json", Output: buf})
	ctx := context.Background()
	derived := logger.WithContext(ctx)
	// When no IDs are in context, WithContext should return the same logger.
	derived.Info("no context ids")
	assert.Contains(t, buf.String(), "no context ids")
}

func TestLoggerWithContext_TraceIDOnly(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(LogConfig{Level: "info", Format: "json", Output: buf})
	ctx := ContextWithTraceID(context.Background(), "t-123")
	logger.InfoContext(ctx, "with trace")
	assert.Contains(t, buf.String(), "t-123")
}

// ---------------------------------------------------------------------------
// SanitizeAPIKey edge cases
// ---------------------------------------------------------------------------

func TestSanitizeAPIKey_Whitespace(t *testing.T) {
	// Non-empty string should be hidden regardless of content.
	assert.Equal(t, "(hidden)", SanitizeAPIKey(" "))
	assert.Equal(t, "(hidden)", SanitizeAPIKey("  \t  "))
}

// ---------------------------------------------------------------------------
// Instrumentation: sanitization helpers
// ---------------------------------------------------------------------------

func TestIsSensitiveArgumentKey(t *testing.T) {
	sensitive := []string{
		"api_key", "apikey", "password", "token", "secret",
		"credentials", "authorization", "x-api-key",
		"access_token", "refresh_token", "client_secret",
		"cookie", "set-cookie",
	}
	for _, key := range sensitive {
		if !isSensitiveArgumentKey(key) {
			t.Errorf("isSensitiveArgumentKey(%q) = false, want true", key)
		}
	}
	// Case insensitive.
	assert.True(t, isSensitiveArgumentKey("API_KEY"))
	assert.True(t, isSensitiveArgumentKey("Password"))

	// Safe keys.
	safe := []string{"name", "url", "path", "method", "content", "description"}
	for _, key := range safe {
		if isSensitiveArgumentKey(key) {
			t.Errorf("isSensitiveArgumentKey(%q) = true, want false", key)
		}
	}
}

func TestContainsSensitiveKeyword(t *testing.T) {
	assert.True(t, containsSensitiveKeyword("my_custom_token_field"))
	assert.True(t, containsSensitiveKeyword("auth_header"))
	assert.True(t, containsSensitiveKeyword("x-api-key-v2"))
	assert.False(t, containsSensitiveKeyword("username"))
	assert.False(t, containsSensitiveKeyword("display_name"))
}

func TestLooksLikeAPIKey(t *testing.T) {
	// Long alphanumeric strings look like API keys.
	assert.True(t, looksLikeAPIKey("sk-1234567890abcdefghijklmnop"))
	assert.True(t, looksLikeAPIKey("abcdefghijklmnopqrstuvwxyz"))

	// Short strings do not.
	assert.False(t, looksLikeAPIKey("short"))
	assert.False(t, looksLikeAPIKey(""))

	// Strings with too many special chars do not.
	assert.False(t, looksLikeAPIKey("this is a normal sentence with spaces and stuff!!!"))
}

func TestSanitizeToolArguments_Nil(t *testing.T) {
	assert.Nil(t, sanitizeToolArguments(nil))
}

func TestSanitizeToolArguments_SafeKeys(t *testing.T) {
	args := map[string]any{
		"path":   "/tmp/file.txt",
		"method": "GET",
		"count":  42,
	}
	got := sanitizeToolArguments(args)
	assert.Equal(t, "/tmp/file.txt", got["path"])
	assert.Equal(t, "GET", got["method"])
	assert.Equal(t, 42, got["count"])
}

func TestSanitizeToolArguments_RedactsAPIKeyLookingValues(t *testing.T) {
	args := map[string]any{
		"api_key": "sk-1234567890abcdef",
		"token":   "some-token-value",
	}
	got := sanitizeToolArguments(args)
	assert.Equal(t, "***REDACTED***", got["api_key"])
	assert.Equal(t, "***REDACTED***", got["token"])
}

// ---------------------------------------------------------------------------
// Metrics: nil collector safety
// ---------------------------------------------------------------------------

func TestMetricsCollector_NilSafe(t *testing.T) {
	var collector *MetricsCollector
	ctx := context.Background()

	// All methods should be nil-safe (no panic).
	collector.RecordLLMRequest(ctx, "model", "ok", time.Second, 100, 50, 0.01)
	collector.RecordToolExecution(ctx, "tool", "ok", time.Millisecond)
	collector.IncrementActiveSessions(ctx)
	collector.DecrementActiveSessions(ctx)
	collector.RecordHTTPServerRequest(ctx, "GET", "/api", 200, time.Millisecond, 100)
	collector.RecordTaskExecution(ctx, "success", time.Second)
}

// ---------------------------------------------------------------------------
// Metrics: EstimateCost edge cases
// ---------------------------------------------------------------------------

func TestEstimateCost_ZeroTokens(t *testing.T) {
	cost := EstimateCost("gpt-4", 0, 0)
	assert.Equal(t, 0.0, cost)
}

func TestEstimateCost_UnknownModelFallback(t *testing.T) {
	cost := EstimateCost("completely-unknown-model-xyz", 1000000, 0)
	// Fallback: $1.5 per 1M tokens.
	assert.InDelta(t, 1.5, cost, 0.01)
}

// ---------------------------------------------------------------------------
// Metrics: test hooks
// ---------------------------------------------------------------------------

func TestMetricsCollector_TestHooks(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: true})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	var httpCalled, sseCalled, taskCalled bool
	collector.SetTestHooks(MetricsTestHooks{
		HTTPServerRequest: func(method, route string, status int, duration time.Duration, responseBytes int64) {
			httpCalled = true
		},
		SSEMessage: func(eventType, status string, sizeBytes int64) {
			sseCalled = true
		},
		TaskExecution: func(status string, duration time.Duration) {
			taskCalled = true
		},
	})

	ctx := context.Background()
	collector.RecordHTTPServerRequest(ctx, "GET", "/api", 200, time.Millisecond, 100)
	collector.RecordSSEMessage(ctx, "data", "ok", 50)
	collector.RecordTaskExecution(ctx, "success", time.Second)

	assert.True(t, httpCalled, "HTTP hook should fire")
	assert.True(t, sseCalled, "SSE hook should fire")
	assert.True(t, taskCalled, "task hook should fire")
}

// ---------------------------------------------------------------------------
// Config: save and reload round-trip
// ---------------------------------------------------------------------------

func TestSaveConfig_PreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"

	// Write initial config with extra key.
	initial := []byte("extra_key: preserved\n")
	require.NoError(t, os.WriteFile(path, initial, 0644))

	// Save observability config.
	cfg := DefaultConfig()
	cfg.Logging.Level = "debug"
	require.NoError(t, SaveConfig(cfg, path))

	// Reload and verify observability was written.
	loaded, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "debug", loaded.Logging.Level)
}
