package config

import (
	"testing"
	"time"
)

func TestCloneStringMapReturnsNilForNilOrEmpty(t *testing.T) {
	if got := cloneStringMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := cloneStringMap(map[string]string{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}
}

func TestCloneStringMapCreatesIndependentCopy(t *testing.T) {
	src := map[string]string{"A": "1"}
	cloned := cloneStringMap(src)
	if len(cloned) != 1 || cloned["A"] != "1" {
		t.Fatalf("unexpected clone: %+v", cloned)
	}

	src["A"] = "2"
	if cloned["A"] != "1" {
		t.Fatalf("expected clone to remain unchanged after source mutation, got %+v", cloned)
	}

	cloned["A"] = "3"
	if src["A"] != "2" {
		t.Fatalf("expected source to remain unchanged after clone mutation, got %+v", src)
	}
}

func TestCloneDurationMapReturnsNilForNilOrEmpty(t *testing.T) {
	if got := cloneDurationMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := cloneDurationMap(map[string]time.Duration{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}
}

func TestCloneDurationMapCreatesIndependentCopy(t *testing.T) {
	src := map[string]time.Duration{"tool.a": time.Second}
	cloned := cloneDurationMap(src)
	if len(cloned) != 1 || cloned["tool.a"] != time.Second {
		t.Fatalf("unexpected clone: %+v", cloned)
	}

	src["tool.a"] = 2 * time.Second
	if cloned["tool.a"] != time.Second {
		t.Fatalf("expected clone to remain unchanged after source mutation, got %+v", cloned)
	}

	cloned["tool.a"] = 3 * time.Second
	if src["tool.a"] != 2*time.Second {
		t.Fatalf("expected source to remain unchanged after clone mutation, got %+v", src)
	}
}

func TestApplyExternalAgentsFileConfigClonesEnvMaps(t *testing.T) {
	cfg := &RuntimeConfig{}
	meta := &Metadata{sources: map[string]ValueSource{}}
	external := &ExternalAgentsFileConfig{
		ClaudeCode: &ClaudeCodeFileConfig{
			Env: map[string]string{"CC_KEY": "one"},
		},
		Codex: &CodexFileConfig{
			Env: map[string]string{"CX_KEY": "two"},
		},
	}

	if err := applyExternalAgentsFileConfig(cfg, meta, external); err != nil {
		t.Fatalf("apply external agents config: %v", err)
	}
	if cfg.ExternalAgents.ClaudeCode.Env["CC_KEY"] != "one" {
		t.Fatalf("unexpected claude env: %+v", cfg.ExternalAgents.ClaudeCode.Env)
	}
	if cfg.ExternalAgents.Codex.Env["CX_KEY"] != "two" {
		t.Fatalf("unexpected codex env: %+v", cfg.ExternalAgents.Codex.Env)
	}
	if meta.Source("external_agents.claude_code.env") != SourceFile {
		t.Fatalf("expected source metadata for claude env")
	}
	if meta.Source("external_agents.codex.env") != SourceFile {
		t.Fatalf("expected source metadata for codex env")
	}

	external.ClaudeCode.Env["CC_KEY"] = "changed"
	if cfg.ExternalAgents.ClaudeCode.Env["CC_KEY"] != "one" {
		t.Fatalf("expected cloned claude env to be independent, got %+v", cfg.ExternalAgents.ClaudeCode.Env)
	}
	cfg.ExternalAgents.Codex.Env["CX_KEY"] = "updated"
	if external.Codex.Env["CX_KEY"] != "two" {
		t.Fatalf("expected source codex env to remain unchanged, got %+v", external.Codex.Env)
	}
}

func TestApplyExternalAgentsFileConfigIgnoresEmptyEnvMaps(t *testing.T) {
	cfg := &RuntimeConfig{
		ExternalAgents: ExternalAgentsConfig{
			ClaudeCode: ClaudeCodeConfig{Env: map[string]string{"KEEP": "1"}},
			Codex:      CodexConfig{Env: map[string]string{"KEEP": "2"}},
		},
	}
	meta := &Metadata{sources: map[string]ValueSource{}}
	external := &ExternalAgentsFileConfig{
		ClaudeCode: &ClaudeCodeFileConfig{Env: map[string]string{}},
		Codex:      &CodexFileConfig{Env: map[string]string{}},
	}

	if err := applyExternalAgentsFileConfig(cfg, meta, external); err != nil {
		t.Fatalf("apply external agents config: %v", err)
	}
	if cfg.ExternalAgents.ClaudeCode.Env["KEEP"] != "1" {
		t.Fatalf("expected existing claude env to remain unchanged, got %+v", cfg.ExternalAgents.ClaudeCode.Env)
	}
	if cfg.ExternalAgents.Codex.Env["KEEP"] != "2" {
		t.Fatalf("expected existing codex env to remain unchanged, got %+v", cfg.ExternalAgents.Codex.Env)
	}
	if meta.Source("external_agents.claude_code.env") != SourceDefault {
		t.Fatalf("did not expect source metadata for empty claude env")
	}
	if meta.Source("external_agents.codex.env") != SourceDefault {
		t.Fatalf("did not expect source metadata for empty codex env")
	}
}

func TestApplyToolPolicyFileConfigClonesPerToolTimeout(t *testing.T) {
	cfg := &RuntimeConfig{}
	meta := &Metadata{sources: map[string]ValueSource{}}
	policy := &ToolPolicyFileConfig{
		Timeout: &ToolTimeoutFileConfig{
			PerTool: map[string]time.Duration{
				"tool.alpha": 5 * time.Second,
			},
		},
	}

	applyToolPolicyFileConfig(cfg, meta, policy)

	if got := cfg.ToolPolicy.Timeout.PerTool["tool.alpha"]; got != 5*time.Second {
		t.Fatalf("unexpected per-tool timeout map: %+v", cfg.ToolPolicy.Timeout.PerTool)
	}
	if meta.Source("tool_policy.timeout.per_tool") != SourceFile {
		t.Fatalf("expected source metadata for per-tool timeout")
	}

	policy.Timeout.PerTool["tool.alpha"] = 9 * time.Second
	if got := cfg.ToolPolicy.Timeout.PerTool["tool.alpha"]; got != 5*time.Second {
		t.Fatalf("expected cloned timeout map to be independent, got %+v", cfg.ToolPolicy.Timeout.PerTool)
	}
	cfg.ToolPolicy.Timeout.PerTool["tool.alpha"] = 11 * time.Second
	if got := policy.Timeout.PerTool["tool.alpha"]; got != 9*time.Second {
		t.Fatalf("expected source timeout map to remain unchanged, got %+v", policy.Timeout.PerTool)
	}
}

func TestApplyToolPolicyFileConfigEmptyPerToolClearsToNil(t *testing.T) {
	cfg := &RuntimeConfig{}
	cfg.ToolPolicy.Timeout.PerTool = map[string]time.Duration{"existing": time.Second}
	meta := &Metadata{sources: map[string]ValueSource{}}
	policy := &ToolPolicyFileConfig{
		Timeout: &ToolTimeoutFileConfig{
			PerTool: map[string]time.Duration{},
		},
	}

	applyToolPolicyFileConfig(cfg, meta, policy)

	if cfg.ToolPolicy.Timeout.PerTool != nil {
		t.Fatalf("expected empty per-tool map override to normalize to nil, got %+v", cfg.ToolPolicy.Timeout.PerTool)
	}
	if meta.Source("tool_policy.timeout.per_tool") != SourceFile {
		t.Fatalf("expected source metadata for per-tool timeout override")
	}
}
