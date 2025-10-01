package errors

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
)

// ErrorType represents the classification of errors for retry logic
type ErrorType int

const (
	// ErrorTypeTransient - retry-able errors
	ErrorTypeTransient ErrorType = iota
	// ErrorTypePermanent - non-retry-able errors
	ErrorTypePermanent
	// ErrorTypeDegraded - can continue with reduced functionality
	ErrorTypeDegraded
)

// TransientError represents an error that can be retried
type TransientError struct {
	Err           error
	RetryAfter    int    // Seconds to wait before retry (from Retry-After header)
	StatusCode    int    // HTTP status code if applicable
	SuggestedWait int    // Suggested wait time in seconds
	Message       string // LLM-friendly message
}

func (e *TransientError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("transient error: %v", e.Err)
}

func (e *TransientError) Unwrap() error {
	return e.Err
}

// PermanentError represents an error that should not be retried
type PermanentError struct {
	Err        error
	StatusCode int    // HTTP status code if applicable
	Message    string // LLM-friendly message
}

func (e *PermanentError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("permanent error: %v", e.Err)
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

// DegradedError represents an error where service can continue with reduced functionality
type DegradedError struct {
	Err             error
	FallbackContent string // Alternative content to return
	Message         string // LLM-friendly message
}

func (e *DegradedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("degraded error: %v", e.Err)
}

func (e *DegradedError) Unwrap() error {
	return e.Err
}

// IsTransient checks if an error is retry-able
func IsTransient(err error) bool {
	if err == nil {
		return false
	}

	// Check if explicitly marked as transient
	var transientErr *TransientError
	if errors.As(err, &transientErr) {
		return true
	}

	// Check if explicitly marked as permanent
	var permanentErr *PermanentError
	if errors.As(err, &permanentErr) {
		return false
	}

	// Network errors (connection refused, timeout, etc.)
	if isNetworkError(err) {
		return true
	}

	// HTTP status codes
	if statusCode := extractHTTPStatusCode(err); statusCode > 0 {
		return isTransientHTTPStatus(statusCode)
	}

	// Syscall errors
	if isSyscallError(err) {
		return true
	}

	// Default: not transient
	return false
}

// IsPermanent checks if an error is non-retry-able
func IsPermanent(err error) bool {
	if err == nil {
		return false
	}

	// Check if explicitly marked as permanent
	var permanentErr *PermanentError
	if errors.As(err, &permanentErr) {
		return true
	}

	// Check if explicitly marked as transient
	var transientErr *TransientError
	if errors.As(err, &transientErr) {
		return false
	}

	// HTTP status codes
	if statusCode := extractHTTPStatusCode(err); statusCode > 0 {
		return isPermanentHTTPStatus(statusCode)
	}

	// Common permanent errors
	errStr := err.Error()
	permanentPatterns := []string{
		"not found",
		"permission denied",
		"invalid",
		"unauthorized",
		"forbidden",
		"bad request",
		"tool not found",
		"file not found",
	}

	lowerErr := strings.ToLower(errStr)
	for _, pattern := range permanentPatterns {
		if strings.Contains(lowerErr, pattern) {
			return true
		}
	}

	return false
}

// IsDegraded checks if an error allows degraded service
func IsDegraded(err error) bool {
	var degradedErr *DegradedError
	return errors.As(err, &degradedErr)
}

// GetErrorType classifies an error
func GetErrorType(err error) ErrorType {
	if err == nil {
		return ErrorTypePermanent // No error is not transient
	}

	if IsDegraded(err) {
		return ErrorTypeDegraded
	}

	if IsTransient(err) {
		return ErrorTypeTransient
	}

	if IsPermanent(err) {
		return ErrorTypePermanent
	}

	// Default to permanent to avoid infinite retries
	return ErrorTypePermanent
}

// FormatForLLM converts technical errors to LLM-friendly actionable messages
func FormatForLLM(err error) string {
	if err == nil {
		return ""
	}

	// Check for custom formatted errors
	var transientErr *TransientError
	if errors.As(err, &transientErr) && transientErr.Message != "" {
		return transientErr.Message
	}

	var permanentErr *PermanentError
	if errors.As(err, &permanentErr) && permanentErr.Message != "" {
		return permanentErr.Message
	}

	var degradedErr *DegradedError
	if errors.As(err, &degradedErr) && degradedErr.Message != "" {
		return degradedErr.Message
	}

	errStr := err.Error()
	lowerErr := strings.ToLower(errStr)

	// Connection refused (Ollama not running)
	if strings.Contains(lowerErr, "connection refused") {
		if strings.Contains(lowerErr, "11434") || strings.Contains(lowerErr, "ollama") {
			return "Ollama server is not running. Please start it with: ollama serve"
		}
		return "Service is not running. Please check if the required service is started."
	}

	// Rate limit errors
	if strings.Contains(lowerErr, "rate limit") || strings.Contains(lowerErr, "429") {
		return "API rate limit reached. The system will automatically retry with backoff. Consider using a cheaper model to reduce request frequency."
	}

	// Timeout errors
	if strings.Contains(lowerErr, "timeout") || strings.Contains(lowerErr, "deadline exceeded") {
		return "Request timed out. The operation may be too complex. Try breaking it into smaller steps or increase the timeout."
	}

	// Network errors
	if strings.Contains(lowerErr, "network") || strings.Contains(lowerErr, "dns") {
		return "Network connectivity issue. Please check your internet connection and try again."
	}

	// Authentication errors
	if strings.Contains(lowerErr, "unauthorized") || strings.Contains(lowerErr, "401") {
		return "Authentication failed. Please check your API key configuration."
	}

	// Permission errors
	if strings.Contains(lowerErr, "permission denied") || strings.Contains(lowerErr, "403") {
		return "Permission denied. You don't have access to this resource."
	}

	// Not found errors
	if strings.Contains(lowerErr, "not found") || strings.Contains(lowerErr, "404") {
		return "Resource not found. Please verify the path or identifier."
	}

	// Invalid request errors
	if strings.Contains(lowerErr, "bad request") || strings.Contains(lowerErr, "400") {
		return "Invalid request. Please check the parameters and try again."
	}

	// Server errors
	if strings.Contains(lowerErr, "500") || strings.Contains(lowerErr, "502") ||
		strings.Contains(lowerErr, "503") || strings.Contains(lowerErr, "internal server error") {
		return "Server error. The service is temporarily unavailable. The system will automatically retry."
	}

	// Default: return original error
	return errStr
}

// Helper functions

func isNetworkError(err error) bool {
	// net.Error with Timeout or Temporary
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Connection errors
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary()
	}

	// Check error strings for common network error patterns
	errStr := strings.ToLower(err.Error())
	networkPatterns := []string{
		"connection refused",
		"timeout",
		"deadline exceeded",
		"network",
		"dns",
		"connection reset",
		"broken pipe",
	}

	for _, pattern := range networkPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

func isSyscallError(err error) bool {
	// Connection reset, broken pipe, etc.
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		switch syscallErr {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.EPIPE,
			syscall.ETIMEDOUT, syscall.ENETUNREACH, syscall.EHOSTUNREACH:
			return true
		}
	}
	return false
}

func isTransientHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	}
	return false
}

func isPermanentHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadRequest, // 400
		http.StatusUnauthorized,        // 401
		http.StatusForbidden,           // 403
		http.StatusNotFound,            // 404
		http.StatusMethodNotAllowed,    // 405
		http.StatusConflict,            // 409
		http.StatusGone,                // 410
		http.StatusUnprocessableEntity: // 422
		return true
	}
	return false
}

func extractHTTPStatusCode(err error) int {
	errStr := err.Error()

	// Try to extract status code from error message
	// Format: "API error 429: ..." or "HTTP 500: ..."
	patterns := []string{
		"status 429", "429", "status 400", "400", "status 401", "401",
		"status 403", "403", "status 404", "404", "status 500", "500",
		"status 502", "502", "status 503", "503", "status 504", "504",
	}

	lowerErr := strings.ToLower(errStr)
	for _, pattern := range patterns {
		if strings.Contains(lowerErr, pattern) {
			// Extract the number
			if strings.HasPrefix(pattern, "status ") {
				code := strings.TrimPrefix(pattern, "status ")
				switch code {
				case "400":
					return 400
				case "401":
					return 401
				case "403":
					return 403
				case "404":
					return 404
				case "429":
					return 429
				case "500":
					return 500
				case "502":
					return 502
				case "503":
					return 503
				case "504":
					return 504
				}
			} else {
				// Just the number
				switch pattern {
				case "400":
					return 400
				case "401":
					return 401
				case "403":
					return 403
				case "404":
					return 404
				case "429":
					return 429
				case "500":
					return 500
				case "502":
					return 502
				case "503":
					return 503
				case "504":
					return 504
				}
			}
		}
	}

	return 0
}

// Helper constructors

// NewTransientError creates a new transient error with LLM-friendly message
func NewTransientError(err error, message string) *TransientError {
	return &TransientError{
		Err:     err,
		Message: message,
	}
}

// NewPermanentError creates a new permanent error with LLM-friendly message
func NewPermanentError(err error, message string) *PermanentError {
	return &PermanentError{
		Err:     err,
		Message: message,
	}
}

// NewDegradedError creates a new degraded error with fallback content
func NewDegradedError(err error, message, fallback string) *DegradedError {
	return &DegradedError{
		Err:             err,
		Message:         message,
		FallbackContent: fallback,
	}
}
