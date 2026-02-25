package config

import (
	"testing"
	"time"

	toolspolicy "alex/internal/infra/tools"
)

func TestCloneStringMapNilAndEmptyReturnNil(t *testing.T) {
	if got := cloneStringMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := cloneStringMap(map[string]string{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}
}

func TestCloneStringMapCreatesIndependentCopy(t *testing.T) {
	src := map[string]string{"mode": "plan"}

	cloned := cloneStringMap(src)
	if len(cloned) != 1 || cloned["mode"] != "plan" {
		t.Fatalf("unexpected clone contents: %+v", cloned)
	}

	src["mode"] = "execute"
	if cloned["mode"] != "plan" {
		t.Fatalf("expected clone to remain unchanged after source mutation, got %+v", cloned)
	}

	cloned["new"] = "value"
	if _, ok := src["new"]; ok {
		t.Fatalf("expected source to remain unchanged after clone mutation, got %+v", src)
	}
}

func TestCloneDurationMapNilAndEmptyReturnNil(t *testing.T) {
	if got := cloneDurationMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := cloneDurationMap(map[string]time.Duration{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}
}

func TestCloneDurationMapCreatesIndependentCopy(t *testing.T) {
	src := map[string]time.Duration{"web_search": 3 * time.Second}

	cloned := cloneDurationMap(src)
	if len(cloned) != 1 || cloned["web_search"] != 3*time.Second {
		t.Fatalf("unexpected clone contents: %+v", cloned)
	}

	src["web_search"] = 9 * time.Second
	if cloned["web_search"] != 3*time.Second {
		t.Fatalf("expected clone to remain unchanged after source mutation, got %+v", cloned)
	}

	cloned["read"] = 4 * time.Second
	if _, ok := src["read"]; ok {
		t.Fatalf("expected source to remain unchanged after clone mutation, got %+v", src)
	}
}

func TestApplyExternalAgentsFileConfigClonesEnvMaps(t *testing.T) {
	cfg := RuntimeConfig{
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	meta := Metadata{sources: map[string]ValueSource{}}
	external := &ExternalAgentsFileConfig{
		ClaudeCode: &ClaudeCodeFileConfig{
			Env: map[string]string{"CLAUDE": "1"},
		},
		Codex: &CodexFileConfig{
			Env: map[string]string{"CODEX": "2"},
		},
		Kimi: &KimiFileConfig{
			Env: map[string]string{"KIMI": "3"},
		},
	}

	if err := applyExternalAgentsFileConfig(&cfg, &meta, external); err != nil {
		t.Fatalf("applyExternalAgentsFileConfig() error = %v", err)
	}

	external.ClaudeCode.Env["CLAUDE"] = "changed"
	external.Codex.Env["CODEX"] = "changed"
	external.Kimi.Env["KIMI"] = "changed"

	if cfg.ExternalAgents.ClaudeCode.Env["CLAUDE"] != "1" {
		t.Fatalf("expected cloned claude_code env map, got %+v", cfg.ExternalAgents.ClaudeCode.Env)
	}
	if cfg.ExternalAgents.Codex.Env["CODEX"] != "2" {
		t.Fatalf("expected cloned codex env map, got %+v", cfg.ExternalAgents.Codex.Env)
	}
	if cfg.ExternalAgents.Kimi.Env["KIMI"] != "3" {
		t.Fatalf("expected cloned kimi env map, got %+v", cfg.ExternalAgents.Kimi.Env)
	}

	cfg.ExternalAgents.ClaudeCode.Env["EXTRA"] = "x"
	if _, ok := external.ClaudeCode.Env["EXTRA"]; ok {
		t.Fatalf("expected source claude_code env map to remain unchanged, got %+v", external.ClaudeCode.Env)
	}

	if got := meta.Source("external_agents.claude_code.env"); got != SourceFile {
		t.Fatalf("expected claude_code env source=file, got %s", got)
	}
	if got := meta.Source("external_agents.codex.env"); got != SourceFile {
		t.Fatalf("expected codex env source=file, got %s", got)
	}
	if got := meta.Source("external_agents.kimi.env"); got != SourceFile {
		t.Fatalf("expected kimi env source=file, got %s", got)
	}
}

func TestApplyExternalAgentsFileConfigIgnoresEmptyEnvMaps(t *testing.T) {
	cfg := RuntimeConfig{
		ExternalAgents: DefaultExternalAgentsConfig(),
	}
	cfg.ExternalAgents.ClaudeCode.Env = map[string]string{"KEEP": "claude"}
	cfg.ExternalAgents.Codex.Env = map[string]string{"KEEP": "codex"}
	cfg.ExternalAgents.Kimi.Env = map[string]string{"KEEP": "kimi"}

	meta := Metadata{sources: map[string]ValueSource{}}
	external := &ExternalAgentsFileConfig{
		ClaudeCode: &ClaudeCodeFileConfig{
			Env: map[string]string{},
		},
		Codex: &CodexFileConfig{
			Env: map[string]string{},
		},
		Kimi: &KimiFileConfig{
			Env: map[string]string{},
		},
	}

	if err := applyExternalAgentsFileConfig(&cfg, &meta, external); err != nil {
		t.Fatalf("applyExternalAgentsFileConfig() error = %v", err)
	}

	if cfg.ExternalAgents.ClaudeCode.Env["KEEP"] != "claude" {
		t.Fatalf("expected empty claude_code env override to be ignored, got %+v", cfg.ExternalAgents.ClaudeCode.Env)
	}
	if cfg.ExternalAgents.Codex.Env["KEEP"] != "codex" {
		t.Fatalf("expected empty codex env override to be ignored, got %+v", cfg.ExternalAgents.Codex.Env)
	}
	if cfg.ExternalAgents.Kimi.Env["KEEP"] != "kimi" {
		t.Fatalf("expected empty kimi env override to be ignored, got %+v", cfg.ExternalAgents.Kimi.Env)
	}

	if got := meta.Source("external_agents.claude_code.env"); got != SourceDefault {
		t.Fatalf("expected claude_code env source to remain default, got %s", got)
	}
	if got := meta.Source("external_agents.codex.env"); got != SourceDefault {
		t.Fatalf("expected codex env source to remain default, got %s", got)
	}
	if got := meta.Source("external_agents.kimi.env"); got != SourceDefault {
		t.Fatalf("expected kimi env source to remain default, got %s", got)
	}
}

func TestApplyToolPolicyFileConfigClonesPerToolTimeoutMap(t *testing.T) {
	cfg := RuntimeConfig{
		ToolPolicy: toolspolicy.DefaultToolPolicyConfig(),
	}
	meta := Metadata{sources: map[string]ValueSource{}}

	perTool := map[string]time.Duration{"web_search": 3 * time.Second}
	policy := &ToolPolicyFileConfig{
		Timeout: &ToolTimeoutFileConfig{
			PerTool: perTool,
		},
	}

	applyToolPolicyFileConfig(&cfg, &meta, policy)

	perTool["web_search"] = 9 * time.Second
	if cfg.ToolPolicy.Timeout.PerTool["web_search"] != 3*time.Second {
		t.Fatalf("expected cloned per_tool timeout map, got %+v", cfg.ToolPolicy.Timeout.PerTool)
	}

	cfg.ToolPolicy.Timeout.PerTool["read"] = 4 * time.Second
	if _, ok := perTool["read"]; ok {
		t.Fatalf("expected source per_tool timeout map to remain unchanged, got %+v", perTool)
	}

	if got := meta.Source("tool_policy.timeout.per_tool"); got != SourceFile {
		t.Fatalf("expected tool_policy.timeout.per_tool source=file, got %s", got)
	}
}

func TestApplyToolPolicyFileConfigEmptyPerToolTimeoutMapClearsValue(t *testing.T) {
	cfg := RuntimeConfig{
		ToolPolicy: toolspolicy.DefaultToolPolicyConfig(),
	}
	cfg.ToolPolicy.Timeout.PerTool = map[string]time.Duration{"web_search": 3 * time.Second}

	meta := Metadata{sources: map[string]ValueSource{}}
	policy := &ToolPolicyFileConfig{
		Timeout: &ToolTimeoutFileConfig{
			PerTool: map[string]time.Duration{},
		},
	}

	applyToolPolicyFileConfig(&cfg, &meta, policy)

	if cfg.ToolPolicy.Timeout.PerTool != nil {
		t.Fatalf("expected empty per_tool timeout map to clear to nil, got %+v", cfg.ToolPolicy.Timeout.PerTool)
	}
	if got := meta.Source("tool_policy.timeout.per_tool"); got != SourceFile {
		t.Fatalf("expected tool_policy.timeout.per_tool source=file, got %s", got)
	}
}
