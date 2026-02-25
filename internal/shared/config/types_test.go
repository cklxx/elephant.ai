package config

import "testing"

func TestDefaultExternalAgentsConfig_CodexDefaultModel_SubscriptionCompatible(t *testing.T) {
	cfg := DefaultExternalAgentsConfig()
	if cfg.Codex.DefaultModel != "gpt-5.2-codex" {
		t.Fatalf("unexpected codex default model: %q", cfg.Codex.DefaultModel)
	}
	if cfg.Kimi.Binary != "kimi" {
		t.Fatalf("unexpected kimi default binary: %q", cfg.Kimi.Binary)
	}
}
