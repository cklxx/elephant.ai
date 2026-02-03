package config

import (
	"fmt"
	"testing"
	"time"
)

func TestAutoEnableExternalAgents_Development_DefaultSources(t *testing.T) {
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		return fmt.Sprintf("/fake/%s", name), nil
	}

	cfg := RuntimeConfig{
		Environment:    "development",
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}

	autoEnableExternalAgents(&cfg, &meta)

	if !cfg.ExternalAgents.Codex.Enabled {
		t.Fatalf("expected codex enabled")
	}
	if meta.Source("external_agents.codex.enabled") != SourceCodexCLI {
		t.Fatalf("unexpected codex source: %s", meta.Source("external_agents.codex.enabled"))
	}

	if !cfg.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code enabled")
	}
	if meta.Source("external_agents.claude_code.enabled") != SourceClaudeCLI {
		t.Fatalf("unexpected claude_code source: %s", meta.Source("external_agents.claude_code.enabled"))
	}
}

func TestAutoEnableExternalAgents_RespectsExplicitConfig(t *testing.T) {
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		return fmt.Sprintf("/fake/%s", name), nil
	}

	cfg := RuntimeConfig{
		Environment:    "development",
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	cfg.ExternalAgents.Codex.Enabled = false
	cfg.ExternalAgents.ClaudeCode.Enabled = false

	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}
	meta.sources["external_agents.codex.enabled"] = SourceFile
	meta.sources["external_agents.claude_code.enabled"] = SourceFile

	autoEnableExternalAgents(&cfg, &meta)

	if cfg.ExternalAgents.Codex.Enabled {
		t.Fatalf("expected codex to remain disabled")
	}
	if cfg.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code to remain disabled")
	}
}

func TestAutoEnableExternalAgents_SkipsNonDevelopment(t *testing.T) {
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		return fmt.Sprintf("/fake/%s", name), nil
	}

	cfg := RuntimeConfig{
		Environment:    "production",
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}

	autoEnableExternalAgents(&cfg, &meta)

	if cfg.ExternalAgents.Codex.Enabled {
		t.Fatalf("expected codex disabled")
	}
	if cfg.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code disabled")
	}
}
