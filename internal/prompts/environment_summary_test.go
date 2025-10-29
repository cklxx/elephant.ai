package prompts

import (
	"strings"
	"testing"
)

func TestFormatEnvironmentSummaryRedactsSensitiveKeys(t *testing.T) {
	host := map[string]string{"API_KEY": "abcdef"}
	sandbox := map[string]string{"PATH": "/bin"}
	summary := FormatEnvironmentSummary(host, sandbox)
	if summary == "" {
		t.Fatalf("expected non-empty summary")
	}
	if contains := strings.Contains(summary, "abcdef"); contains {
		t.Fatalf("expected sensitive value redacted, got %s", summary)
	}
	if !strings.Contains(summary, "***") {
		t.Fatalf("expected redaction placeholder")
	}
}
