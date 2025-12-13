package bootstrap

import "testing"

func TestBuildAnalyticsClient_NoKeyUsesNoop(t *testing.T) {
	client, cleanup := BuildAnalyticsClient(AnalyticsConfig{}, nil)
	if client == nil {
		t.Fatalf("expected client, got nil")
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup func, got nil")
	}
	cleanup()
}
