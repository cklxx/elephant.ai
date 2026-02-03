package config

import (
	"os/exec"
	"strings"
)

var lookPath = exec.LookPath

func autoEnableExternalAgents(cfg *RuntimeConfig, meta *Metadata) {
	if cfg == nil || meta == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Environment), "development") {
		return
	}

	if meta.Source("external_agents.codex.enabled") == SourceDefault {
		binary := strings.TrimSpace(cfg.ExternalAgents.Codex.Binary)
		if binary == "" {
			binary = "codex"
		}
		if _, err := lookPath(binary); err == nil {
			cfg.ExternalAgents.Codex.Enabled = true
			meta.sources["external_agents.codex.enabled"] = SourceCodexCLI
		}
	}

	if meta.Source("external_agents.claude_code.enabled") == SourceDefault {
		binary := strings.TrimSpace(cfg.ExternalAgents.ClaudeCode.Binary)
		if binary == "" {
			binary = "claude"
		}
		if _, err := lookPath(binary); err == nil {
			cfg.ExternalAgents.ClaudeCode.Enabled = true
			meta.sources["external_agents.claude_code.enabled"] = SourceClaudeCLI
		}
	}
}
