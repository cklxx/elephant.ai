package llm

import (
	"errors"
	"testing"

	alexerrors "alex/internal/shared/errors"
)

func TestExtractRequestIntent(t *testing.T) {
	t.Parallel()

	if got := extractRequestIntent(map[string]any{"intent": "task_preanalysis"}); got != "task_preanalysis" {
		t.Fatalf("expected explicit intent, got %q", got)
	}
	if got := extractRequestIntent(map[string]any{"operation": "memory_capture"}); got != "memory_capture" {
		t.Fatalf("expected operation fallback, got %q", got)
	}
	if got := extractRequestIntent(nil); got != "" {
		t.Fatalf("expected empty intent for nil metadata, got %q", got)
	}
}

func TestParseUpstreamError(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":{"type":"rate_limit_error","message":"quota exceeded","code":"rate_limit_exceeded"}}`)
	errType, errCode, errMessage := parseUpstreamError(body)
	if errType != "rate_limit_error" {
		t.Fatalf("expected rate_limit_error type, got %q", errType)
	}
	if errCode != "rate_limit_exceeded" {
		t.Fatalf("expected rate_limit_exceeded code, got %q", errCode)
	}
	if errMessage != "quota exceeded" {
		t.Fatalf("expected quota exceeded message, got %q", errMessage)
	}
}

func TestClassifyFailureError(t *testing.T) {
	t.Parallel()

	transient := alexerrors.NewTransientError(errors.New("429"), "rate limited")
	if got := classifyFailureError(transient); got != "transient" {
		t.Fatalf("expected transient classification, got %q", got)
	}

	permanent := alexerrors.NewPermanentError(errors.New("401"), "unauthorized")
	if got := classifyFailureError(permanent); got != "permanent" {
		t.Fatalf("expected permanent classification, got %q", got)
	}

	if got := classifyFailureError(nil); got != "none" {
		t.Fatalf("expected none classification for nil error, got %q", got)
	}
}

