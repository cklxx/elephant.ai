package bootstrap

import (
	"testing"
)

func TestCaptureHostEnvironment_ReturnsSummary(t *testing.T) {
	env, summary := CaptureHostEnvironment(5)
	if summary == "" {
		t.Fatalf("expected non-empty summary")
	}
	if env == nil || env["summary"] == "" {
		t.Fatalf("expected env summary map to be populated")
	}
}
