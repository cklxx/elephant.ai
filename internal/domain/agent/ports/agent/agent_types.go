package agent

import "strings"

// Agent type constants used across team orchestration.
const (
	AgentTypeCodex      = "codex"
	AgentTypeClaudeCode = "claude_code"
	AgentTypeKimi       = "kimi"
	AgentTypeGenericCLI = "generic_cli"
	AgentTypeInternal   = "internal"
)

// IsCodingExternalAgent returns true for agent types representing external
// coding CLIs that benefit from coding-specific defaults (verify, merge,
// retry, workspace mode).
func IsCodingExternalAgent(agentType string) bool {
	switch CanonicalAgentType(agentType) {
	case AgentTypeCodex, AgentTypeClaudeCode, AgentTypeKimi, AgentTypeGenericCLI:
		return true
	default:
		return false
	}
}

// CanonicalAgentType normalizes agent type aliases to their canonical constant.
func CanonicalAgentType(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch strings.ToLower(trimmed) {
	case "":
		return ""
	case "internal":
		return AgentTypeInternal
	case "generic_cli", "generic-cli", "generic":
		return AgentTypeGenericCLI
	case "codex":
		return AgentTypeCodex
	case "kimi", "kimi_cli", "kimi-cli", "k2", "kimi cli":
		return AgentTypeKimi
	case "claude_code", "claude-code", "claude code":
		return AgentTypeClaudeCode
	default:
		return trimmed
	}
}
