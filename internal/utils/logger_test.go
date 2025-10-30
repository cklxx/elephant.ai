package utils

import (
	"fmt"
	"strings"
	"testing"

	"alex/internal/security/redaction"
)

func TestSanitizeLogLineRedactsAPIKeyAssignment(t *testing.T) {
	line := "2024-10-10 [INFO] [ALEX] sample.go:10 - apiKey=sk-test12345678901234567890\n"
	sanitized := sanitizeLogLine(line)
	expected := fmt.Sprintf("2024-10-10 [INFO] [ALEX] sample.go:10 - apiKey=%s\n", redaction.Placeholder)
	if sanitized != expected {
		t.Fatalf("expected %q, got %q", expected, sanitized)
	}
}

func TestSanitizeLogLineRedactsBearerToken(t *testing.T) {
	line := "token Authorization: Bearer sk-secret-token-here"
	sanitized := sanitizeLogLine(line)
	expected := fmt.Sprintf("token Authorization: Bearer %s", redaction.Placeholder)
	if sanitized != expected {
		t.Fatalf("expected %q, got %q", expected, sanitized)
	}
}

func TestSanitizeLogLineRedactsStandaloneSecret(t *testing.T) {
	line := "random ghp_abcd1234efgh5678ijkl9012mnop3456 value"
	sanitized := sanitizeLogLine(line)
	if sanitized == line {
		t.Fatalf("expected token to be redacted, got %q", sanitized)
	}
	if !containsPlaceholder(sanitized) {
		t.Fatalf("expected placeholder in sanitized line: %q", sanitized)
	}
}

func containsPlaceholder(line string) bool {
	return strings.Contains(line, redaction.Placeholder)
}
