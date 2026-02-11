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

func TestAutoEnableExternalAgents_Production_DefaultSources(t *testing.T) {
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

	if !cfg.ExternalAgents.Codex.Enabled {
		t.Fatalf("expected codex enabled in production when binary exists")
	}
	if !cfg.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code enabled in production when binary exists")
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

func TestAutoEnableExternalAgents_SkipsWhenBinaryMissing(t *testing.T) {
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		return "", fmt.Errorf("binary %s not found", name)
	}

	cfg := RuntimeConfig{
		Environment:    "development",
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

func TestAutoEnableExternalAgents_UsesFallbackBinaryCandidates(t *testing.T) {
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		if name == "claude-code" {
			return "/fake/claude-code", nil
		}
		return "", fmt.Errorf("binary %s not found", name)
	}

	cfg := RuntimeConfig{
		Environment:    "development",
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	cfg.ExternalAgents.ClaudeCode.Binary = "claude"
	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}

	autoEnableExternalAgents(&cfg, &meta)

	if cfg.ExternalAgents.Codex.Enabled {
		t.Fatalf("expected codex disabled when binary missing")
	}
	if !cfg.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code enabled via fallback candidate")
	}
	if cfg.ExternalAgents.ClaudeCode.Binary != "claude-code" {
		t.Fatalf("expected claude_code binary updated to detected fallback, got %q", cfg.ExternalAgents.ClaudeCode.Binary)
	}
	if meta.Source("external_agents.claude_code.enabled") != SourceClaudeCLI {
		t.Fatalf("unexpected claude_code source: %s", meta.Source("external_agents.claude_code.enabled"))
	}
	if meta.Source("external_agents.claude_code.binary") != SourceClaudeCLI {
		t.Fatalf("expected claude_code binary source set to claude cli, got %s", meta.Source("external_agents.claude_code.binary"))
	}
}

func TestAutoEnableExternalAgents_DoesNotFallbackWhenBinaryExplicitlyConfigured(t *testing.T) {
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		if name == "claude-code" {
			return "/fake/claude-code", nil
		}
		return "", fmt.Errorf("binary %s not found", name)
	}

	cfg := RuntimeConfig{
		Environment:    "development",
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	cfg.ExternalAgents.ClaudeCode.Binary = "custom-claude"
	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}
	meta.sources["external_agents.claude_code.binary"] = SourceFile

	autoEnableExternalAgents(&cfg, &meta)

	if cfg.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code disabled when explicit binary is missing")
	}
	if cfg.ExternalAgents.ClaudeCode.Binary != "custom-claude" {
		t.Fatalf("expected configured binary to remain unchanged, got %q", cfg.ExternalAgents.ClaudeCode.Binary)
	}
}

func TestAutoEnableExternalAgents_ExplicitBinaryStillAutoEnablesWhenPresent(t *testing.T) {
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(name string) (string, error) {
		if name == "custom-claude" {
			return "/fake/custom-claude", nil
		}
		return "", fmt.Errorf("binary %s not found", name)
	}

	cfg := RuntimeConfig{
		Environment:    "development",
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	cfg.ExternalAgents.ClaudeCode.Binary = "custom-claude"
	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}
	meta.sources["external_agents.claude_code.binary"] = SourceFile

	autoEnableExternalAgents(&cfg, &meta)

	if !cfg.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code enabled when explicit configured binary exists")
	}
	if cfg.ExternalAgents.ClaudeCode.Binary != "custom-claude" {
		t.Fatalf("expected configured binary to remain unchanged, got %q", cfg.ExternalAgents.ClaudeCode.Binary)
	}
	if meta.Source("external_agents.claude_code.binary") != SourceFile {
		t.Fatalf("expected claude_code binary source to remain file, got %s", meta.Source("external_agents.claude_code.binary"))
	}
}
