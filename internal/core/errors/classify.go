package errors

import (
	"fmt"
	"strings"
)

// ClassifiedError wraps an error with a Kind and optional context.
type ClassifiedError struct {
	Kind    ErrorKind
	Err     error
	Message string         // human-friendly message
	Context map[string]any
}

func (e *ClassifiedError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("[%s] %s: %v", e.Kind, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %v", e.Kind, e.Err)
}

func (e *ClassifiedError) Unwrap() error {
	return e.Err
}

// Classifier is a function that attempts to classify an error.
// Returns the kind and true if it can classify, or Unknown and false otherwise.
type Classifier func(err error) (ErrorKind, bool)

// Classify runs the error through classifiers in order, returning the first match.
// If no classifier matches, returns Unknown.
func Classify(err error, classifiers ...Classifier) *ClassifiedError {
	if err == nil {
		return nil
	}
	for _, c := range classifiers {
		if kind, ok := c(err); ok {
			return &ClassifiedError{
				Kind: kind,
				Err:  err,
			}
		}
	}
	return &ClassifiedError{
		Kind: Unknown,
		Err:  err,
	}
}

// HTTPClassifier classifies errors based on HTTP status codes in error messages.
func HTTPClassifier() Classifier {
	return func(err error) (ErrorKind, bool) {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "400") || strings.Contains(msg, "422"):
			return InvalidInput, true
		case strings.Contains(msg, "401") || strings.Contains(msg, "403"):
			return Config, true
		case strings.Contains(msg, "404"):
			return NotFound, true
		case strings.Contains(msg, "429") || strings.Contains(msg, "503") || strings.Contains(msg, "502"):
			return Temporary, true
		case strings.Contains(msg, "500"):
			return Provider, true
		default:
			return Unknown, false
		}
	}
}

// NetworkClassifier classifies network-related errors as Temporary.
func NetworkClassifier() Classifier {
	return func(err error) (ErrorKind, bool) {
		msg := strings.ToLower(err.Error())
		networkIndicators := []string{
			"connection refused",
			"connection reset",
			"timeout",
			"timed out",
			"no such host",
			"dns",
			"eof",
			"broken pipe",
			"network unreachable",
			"i/o timeout",
		}
		for _, indicator := range networkIndicators {
			if strings.Contains(msg, indicator) {
				return Temporary, true
			}
		}
		return Unknown, false
	}
}

// ToolClassifier classifies tool-related errors.
func ToolClassifier() Classifier {
	return func(err error) (ErrorKind, bool) {
		msg := strings.ToLower(err.Error())
		toolIndicators := []string{
			"tool",
			"function call",
			"plugin",
			"extension",
		}
		for _, indicator := range toolIndicators {
			if strings.Contains(msg, indicator) {
				return Tool, true
			}
		}
		return Unknown, false
	}
}
