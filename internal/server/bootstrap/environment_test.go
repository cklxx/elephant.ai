package bootstrap

import (
	"context"
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

func TestCaptureSandboxEnvironment_NilManagerIsNoop(t *testing.T) {
	env, summary, _, ok := CaptureSandboxEnvironment(context.Background(), nil, 5, nil)
	if ok {
		t.Fatalf("expected ok=false")
	}
	if env != nil {
		t.Fatalf("expected nil env, got %#v", env)
	}
	if summary != "" {
		t.Fatalf("expected empty summary, got %q", summary)
	}
}
