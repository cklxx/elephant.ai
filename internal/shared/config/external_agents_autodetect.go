package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var lookPath = exec.LookPath
var userHomeDir = os.UserHomeDir

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
		if _, resolved, ok := detectFirstAvailableBinary(candidates); ok {
			cfg.ExternalAgents.Codex.Enabled = true
			meta.sources["external_agents.codex.enabled"] = SourceCodexCLI
			if meta.Source("external_agents.codex.binary") == SourceDefault {
				cfg.ExternalAgents.Codex.Binary = resolved
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
		if _, resolved, ok := detectFirstAvailableBinary(candidates); ok {
			cfg.ExternalAgents.ClaudeCode.Enabled = true
			meta.sources["external_agents.claude_code.enabled"] = SourceClaudeCLI
			if meta.Source("external_agents.claude_code.binary") == SourceDefault {
				cfg.ExternalAgents.ClaudeCode.Binary = resolved
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

func detectFirstAvailableBinary(candidates []string) (binary string, resolved string, ok bool) {
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if path, err := resolveBinaryPath(trimmed); err == nil {
			return trimmed, path, true
		}
	}
	return "", "", false
}

func resolveBinaryPath(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		if isExecutable(binary) {
			return binary, nil
		}
		return "", fmt.Errorf("binary not executable: %s", binary)
	}
	if path, err := lookPath(binary); err == nil {
		return path, nil
	}
	for _, dir := range fallbackBinaryDirs() {
		candidate := filepath.Join(dir, binary)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("binary not found: %s", binary)
}

func fallbackBinaryDirs() []string {
	dirs := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
	}
	home, err := userHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		dirs = append([]string{
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".npm", "bin"),
		}, dirs...)
	}
	return dirs
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
