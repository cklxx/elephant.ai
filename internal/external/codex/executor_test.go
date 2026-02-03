package codex

import (
	"strings"
	"testing"
)

type fakeExitError struct {
	code int
}

func (f fakeExitError) Error() string { return "process exit" }
func (f fakeExitError) ExitCode() int { return f.code }

func TestFormatProcessError_IncludesTailAndExit(t *testing.T) {
	msg := formatProcessError("codex", fakeExitError{code: 3}, "api key missing")
	if !strings.Contains(msg, "stderr tail") {
		t.Fatalf("expected stderr tail in error, got %q", msg)
	}
	if !strings.Contains(msg, "exit=3") {
		t.Fatalf("expected exit code in error, got %q", msg)
	}
}

func TestMaybeAppendAuthHintCodex(t *testing.T) {
	base := "codex exited: process exit"
	msg := maybeAppendAuthHintCodex(base, "API key missing")
	if !strings.Contains(strings.ToLower(msg), "api key") {
		t.Fatalf("expected auth hint appended, got %q", msg)
	}
}
