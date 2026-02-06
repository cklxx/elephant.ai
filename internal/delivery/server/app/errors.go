package app

import (
	"errors"
	"fmt"
)

// Domain error sentinels for the server application layer.
// These enable consistent HTTP status mapping via errors.Is().

var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrValidation indicates invalid input from the caller.
	ErrValidation = errors.New("validation error")

	// ErrUnavailable indicates a required dependency is not configured or ready.
	ErrUnavailable = errors.New("service unavailable")

	// ErrConflict indicates a state conflict (e.g., cancel on completed task).
	ErrConflict = errors.New("conflict")
)

// NotFoundError wraps ErrNotFound with a descriptive message.
func NotFoundError(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrNotFound)
}

// ValidationError wraps ErrValidation with a descriptive message.
func ValidationError(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrValidation)
}

// UnavailableError wraps ErrUnavailable with a descriptive message.
func UnavailableError(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrUnavailable)
}

// ConflictError wraps ErrConflict with a descriptive message.
func ConflictError(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrConflict)
}
