package bootstrap

import (
	"testing"

	runtimeconfig "alex/internal/shared/config"
)

func TestBuildAnalyticsClient_NoKeyUsesNoop(t *testing.T) {
	client, cleanup := BuildAnalyticsClient(runtimeconfig.AnalyticsConfig{}, nil)
	if client == nil {
		t.Fatalf("expected client, got nil")
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup func, got nil")
	}
	cleanup()
}
