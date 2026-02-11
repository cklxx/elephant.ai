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

	if meta.Source("external_agents.codex.enabled") == SourceDefault {
		candidates := autoDetectBinaryCandidates(
			meta.Source("external_agents.codex.binary"),
			strings.TrimSpace(cfg.ExternalAgents.Codex.Binary),
			"codex",
		)
		if binary, ok := detectFirstAvailableBinary(candidates); ok {
			cfg.ExternalAgents.Codex.Enabled = true
			meta.sources["external_agents.codex.enabled"] = SourceCodexCLI
			if meta.Source("external_agents.codex.binary") == SourceDefault {
				cfg.ExternalAgents.Codex.Binary = binary
				meta.sources["external_agents.codex.binary"] = SourceCodexCLI
			}
		}
	}

	if meta.Source("external_agents.claude_code.enabled") == SourceDefault {
		candidates := autoDetectBinaryCandidates(
			meta.Source("external_agents.claude_code.binary"),
			strings.TrimSpace(cfg.ExternalAgents.ClaudeCode.Binary),
			"claude",
			"claude-code",
		)
		if binary, ok := detectFirstAvailableBinary(candidates); ok {
			cfg.ExternalAgents.ClaudeCode.Enabled = true
			meta.sources["external_agents.claude_code.enabled"] = SourceClaudeCLI
			if meta.Source("external_agents.claude_code.binary") == SourceDefault {
				cfg.ExternalAgents.ClaudeCode.Binary = binary
				meta.sources["external_agents.claude_code.binary"] = SourceClaudeCLI
			}
		}
	}
}

func autoDetectBinaryCandidates(binarySource ValueSource, preferred string, fallbacks ...string) []string {
	if binarySource != SourceDefault {
		return preferredBinaryCandidates(preferred)
	}
	return preferredBinaryCandidates(preferred, fallbacks...)
}

func preferredBinaryCandidates(preferred string, fallbacks ...string) []string {
	out := make([]string, 0, 1+len(fallbacks))
	if trimmed := strings.TrimSpace(preferred); trimmed != "" {
		out = append(out, trimmed)
	}
	for _, fallback := range fallbacks {
		trimmed := strings.TrimSpace(fallback)
		if trimmed == "" {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if strings.EqualFold(existing, trimmed) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, trimmed)
		}
	}
	return out
}

func detectFirstAvailableBinary(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if _, err := lookPath(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}
