package errors

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "explicit transient error",
			err:      NewTransientError(errors.New("test"), "transient"),
			expected: true,
		},
		{
			name:     "explicit permanent error",
			err:      NewPermanentError(errors.New("test"), "permanent"),
			expected: false,
		},
		{
			name:     "rate limit 429",
			err:      fmt.Errorf("API error 429: rate limit exceeded"),
			expected: true,
		},
		{
			name:     "server error 500",
			err:      fmt.Errorf("HTTP 500: internal server error"),
			expected: true,
		},
		{
			name:     "server error 502",
			err:      fmt.Errorf("502 bad gateway"),
			expected: true,
		},
		{
			name:     "server error 503",
			err:      fmt.Errorf("503 service unavailable"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      fmt.Errorf("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      fmt.Errorf("dial tcp 127.0.0.1:11434: connect: connection refused"),
			expected: true,
		},
		{
			name:     "unauthorized 401",
			err:      fmt.Errorf("HTTP 401: unauthorized"),
			expected: false,
		},
		{
			name:     "not found 404",
			err:      fmt.Errorf("HTTP 404: not found"),
			expected: false,
		},
		{
			name:     "bad request 400",
			err:      fmt.Errorf("HTTP 400: bad request"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransient(tt.err)
			if result != tt.expected {
				t.Errorf("IsTransient(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsPermanent(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "explicit permanent error",
			err:      NewPermanentError(errors.New("test"), "permanent"),
			expected: true,
		},
		{
			name:     "explicit transient error",
			err:      NewTransientError(errors.New("test"), "transient"),
			expected: false,
		},
		{
			name:     "unauthorized 401",
			err:      fmt.Errorf("HTTP 401: unauthorized"),
			expected: true,
		},
		{
			name:     "forbidden 403",
			err:      fmt.Errorf("HTTP 403: forbidden"),
			expected: true,
		},
		{
			name:     "not found 404",
			err:      fmt.Errorf("HTTP 404: not found"),
			expected: true,
		},
		{
			name:     "bad request 400",
			err:      fmt.Errorf("HTTP 400: bad request"),
			expected: true,
		},
		{
			name:     "file not found",
			err:      fmt.Errorf("file not found: /path/to/file"),
			expected: true,
		},
		{
			name:     "permission denied",
			err:      fmt.Errorf("permission denied"),
			expected: true,
		},
		{
			name:     "rate limit 429",
			err:      fmt.Errorf("HTTP 429: rate limit exceeded"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPermanent(tt.err)
			if result != tt.expected {
				t.Errorf("IsPermanent(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{
			name:     "transient error",
			err:      NewTransientError(errors.New("test"), "transient"),
			expected: ErrorTypeTransient,
		},
		{
			name:     "permanent error",
			err:      NewPermanentError(errors.New("test"), "permanent"),
			expected: ErrorTypePermanent,
		},
		{
			name:     "degraded error",
			err:      NewDegradedError(errors.New("test"), "degraded", "fallback"),
			expected: ErrorTypeDegraded,
		},
		{
			name:     "rate limit",
			err:      fmt.Errorf("API error 429: rate limit"),
			expected: ErrorTypeTransient,
		},
		{
			name:     "unauthorized",
			err:      fmt.Errorf("HTTP 401: unauthorized"),
			expected: ErrorTypePermanent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorType(tt.err)
			if result != tt.expected {
				t.Errorf("GetErrorType(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestFormatForLLM(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string // Expected substring in output
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "",
		},
		{
			name:     "custom transient message",
			err:      NewTransientError(errors.New("test"), "Custom transient message"),
			contains: "Custom transient message",
		},
		{
			name:     "llama.cpp connection refused",
			err:      fmt.Errorf("dial tcp 127.0.0.1:8082: connect: connection refused"),
			contains: "llama.cpp server is not running",
		},
		{
			name:     "rate limit",
			err:      fmt.Errorf("API error 429: rate limit exceeded"),
			contains: "rate limit reached",
		},
		{
			name:     "timeout",
			err:      fmt.Errorf("context deadline exceeded"),
			contains: "timed out",
		},
		{
			name:     "unauthorized",
			err:      fmt.Errorf("HTTP 401: unauthorized"),
			contains: "Authentication failed",
		},
		{
			name:     "not found",
			err:      fmt.Errorf("HTTP 404: not found"),
			contains: "not found",
		},
		{
			name:     "server error 500",
			err:      fmt.Errorf("HTTP 500: internal server error"),
			contains: "Server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatForLLM(tt.err)
			if tt.contains != "" && result == "" {
				t.Errorf("FormatForLLM(%v) returned empty string", tt.err)
			}
			if tt.contains != "" {
				// Case-insensitive contains check
				if !containsIgnoreCase(result, tt.contains) {
					t.Errorf("FormatForLLM(%v) = %q, want to contain %q", tt.err, result, tt.contains)
				}
			}
		})
	}
}

func TestNetworkErrorDetection(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error",
			err:      &mockNetError{timeout: true},
			expected: true,
		},
		{
			name:     "temporary error",
			err:      &mockNetError{temporary: true},
			expected: true,
		},
		{
			name:     "syscall connection refused",
			err:      syscall.ECONNREFUSED,
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "connection refused string",
			err:      fmt.Errorf("dial tcp 127.0.0.1:11434: connect: connection refused"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransient(tt.err)
			if result != tt.expected {
				t.Errorf("IsTransient(%v) = %v, want %v (network detection)", tt.err, result, tt.expected)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	baseErr := errors.New("base error")

	t.Run("transient error wrapping", func(t *testing.T) {
		wrapped := NewTransientError(baseErr, "transient message")
		if !errors.Is(wrapped, baseErr) {
			t.Errorf("TransientError should wrap base error")
		}
	})

	t.Run("permanent error wrapping", func(t *testing.T) {
		wrapped := NewPermanentError(baseErr, "permanent message")
		if !errors.Is(wrapped, baseErr) {
			t.Errorf("PermanentError should wrap base error")
		}
	})

	t.Run("degraded error wrapping", func(t *testing.T) {
		wrapped := NewDegradedError(baseErr, "degraded message", "fallback")
		if !errors.Is(wrapped, baseErr) {
			t.Errorf("DegradedError should wrap base error")
		}
	})
}

func TestExtractHTTPStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "400 bad request",
			err:      fmt.Errorf("API error 400: bad request"),
			expected: 400,
		},
		{
			name:     "429 rate limit",
			err:      fmt.Errorf("HTTP 429: Too Many Requests"),
			expected: 429,
		},
		{
			name:     "500 internal server error",
			err:      fmt.Errorf("status 500"),
			expected: 500,
		},
		{
			name:     "no status code",
			err:      fmt.Errorf("generic error"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHTTPStatusCode(tt.err)
			if result != tt.expected {
				t.Errorf("extractHTTPStatusCode(%v) = %d, want %d", tt.err, result, tt.expected)
			}
		})
	}
}

// Mock implementations for testing

type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

// Helper functions

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
