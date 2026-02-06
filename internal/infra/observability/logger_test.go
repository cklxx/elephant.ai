package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	id "alex/internal/shared/utils/id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config LogConfig
	}{
		{
			name: "json format",
			config: LogConfig{
				Level:  "info",
				Format: "json",
			},
		},
		{
			name: "text format",
			config: LogConfig{
				Level:  "debug",
				Format: "text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tt.config.Output = buf

			logger := NewLogger(tt.config)
			require.NotNil(t, logger)

			// Test logging
			logger.Info("test message", "key", "value")
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestLoggerLevels(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(LogConfig{
		Level:  "warn",
		Format: "json",
		Output: buf,
	})

	// Debug and Info should not appear (below warn level)
	logger.Debug("debug message")
	logger.Info("info message")
	assert.Empty(t, buf.String())

	// Warn should appear
	logger.Warn("warn message")
	assert.Contains(t, buf.String(), "warn message")

	// Error should appear
	buf.Reset()
	logger.Error("error message")
	assert.Contains(t, buf.String(), "error message")
}

func TestLoggerWithContext(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: buf,
	})

	// Add context values
	ctx := context.Background()
	ctx = ContextWithTraceID(ctx, "trace-123")
	ctx = ContextWithSessionID(ctx, "session-456")
	ctx = id.WithRunID(ctx, "task-789")
	ctx = id.WithParentRunID(ctx, "parent-000")

	logger.InfoContext(ctx, "test message")

	// Check output was written
	output := buf.String()
	require.NotEmpty(t, output, "No log output was written")

	// Parse JSON output
	var logEntry map[string]any
	err := json.NewDecoder(buf).Decode(&logEntry)
	require.NoError(t, err)

	assert.Contains(t, output, "trace-123")
	assert.Contains(t, output, "session-456")
	assert.Contains(t, output, "task-789")
	assert.Contains(t, output, "parent-000")
}

func TestSanitizeAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "empty key",
			key:      "",
			expected: "(not set)",
		},
		{
			name:     "short key",
			key:      "short",
			expected: "(hidden)",
		},
		{
			name:     "long key",
			key:      "sk-1234567890abcdefghijklmnop",
			expected: "(hidden)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeAPIKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test trace ID
	ctx = ContextWithTraceID(ctx, "trace-123")
	assert.Equal(t, "trace-123", TraceIDFromContext(ctx))

	// Test session ID
	ctx = ContextWithSessionID(ctx, "session-456")
	assert.Equal(t, "session-456", SessionIDFromContext(ctx))

	// Test empty context
	emptyCtx := context.Background()
	assert.Empty(t, TraceIDFromContext(emptyCtx))
	assert.Empty(t, SessionIDFromContext(emptyCtx))
}

func TestLoggerWith(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: buf,
	})

	// Add persistent fields
	loggerWithFields := logger.With("service", "alex", "version", "1.0")
	loggerWithFields.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "alex")
	assert.Contains(t, output, "1.0")
}

func TestLoggerJSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: buf,
	})

	logger.Info("test message", "key1", "value1", "key2", 42)

	// Verify valid JSON
	var logEntry map[string]any
	err := json.NewDecoder(buf).Decode(&logEntry)
	require.NoError(t, err)

	assert.Equal(t, "test message", logEntry["msg"])
	assert.NotEmpty(t, logEntry["time"])
	assert.Equal(t, "INFO", strings.ToUpper(logEntry["level"].(string)))
}
