package coding

import (
	"strings"

	core "alex/internal/domain/agent/ports"
	"alex/internal/shared/executioncontrol"
	"alex/internal/shared/utils"
)

const (
	executionModePlan = "plan"

	autonomySemi = "semi"
	autonomyFull = "full"
)

const (
	claudePlanAllowedTools = "Read,Glob,Grep,WebSearch"
	claudeSemiAllowedTools = "Read,Glob,Grep,WebSearch,Write,Edit,Bash"
)

// applyExecutionControls normalizes cross-agent controls into agent-specific config keys.
func applyExecutionControls(agentType, mode, level string, config map[string]string) map[string]string {
	cfg := core.CloneStringMap(config)
	if cfg == nil {
		cfg = make(map[string]string)
	}

	modeRaw := strings.TrimSpace(mode)
	levelRaw := strings.TrimSpace(level)
	if modeRaw == "" {
		modeRaw = strings.TrimSpace(cfg["execution_mode"])
	}
	if levelRaw == "" {
		levelRaw = strings.TrimSpace(cfg["autonomy_level"])
	}
	if modeRaw == "" && levelRaw == "" {
		return cfg
	}

	normalizedMode := executioncontrol.NormalizeExecutionMode(modeRaw)
	normalizedLevel := executioncontrol.NormalizeAutonomyLevel(levelRaw)
	cfg["execution_mode"] = normalizedMode
	cfg["autonomy_level"] = normalizedLevel

	switch utils.TrimLower(agentType) {
	case "codex", "kimi", "generic_cli":
		applyCodexExecutionControls(cfg, normalizedMode, normalizedLevel)
	case "claude_code":
		applyClaudeExecutionControls(cfg, normalizedMode, normalizedLevel)
	}

	return cfg
}

func applyCodexExecutionControls(cfg map[string]string, mode, level string) {
	if mode == executionModePlan {
		cfg["sandbox"] = "read-only"
		cfg["approval_policy"] = "never"
		return
	}

	switch level {
	case autonomyFull:
		if utils.IsBlank(cfg["sandbox"]) {
			cfg["sandbox"] = "danger-full-access"
		}
		if utils.IsBlank(cfg["approval_policy"]) {
			cfg["approval_policy"] = "never"
		}
	case autonomySemi:
		if utils.IsBlank(cfg["sandbox"]) {
			cfg["sandbox"] = "workspace-write"
		}
		if utils.IsBlank(cfg["approval_policy"]) {
			cfg["approval_policy"] = "on-failure"
		}
	default:
		if utils.IsBlank(cfg["sandbox"]) {
			cfg["sandbox"] = "workspace-write"
		}
		if utils.IsBlank(cfg["approval_policy"]) {
			cfg["approval_policy"] = "on-request"
		}
	}
}

func applyClaudeExecutionControls(cfg map[string]string, mode, level string) {
	if mode == executionModePlan {
		cfg["mode"] = "autonomous"
		if utils.IsBlank(cfg["allowed_tools"]) {
			cfg["allowed_tools"] = claudePlanAllowedTools
		}
		return
	}

	switch level {
	case autonomyFull:
		if utils.IsBlank(cfg["mode"]) {
			cfg["mode"] = "autonomous"
		}
		if utils.IsBlank(cfg["allowed_tools"]) {
			cfg["allowed_tools"] = "*"
		}
	case autonomySemi:
		if utils.IsBlank(cfg["mode"]) {
			cfg["mode"] = "autonomous"
		}
		if utils.IsBlank(cfg["allowed_tools"]) {
			cfg["allowed_tools"] = claudeSemiAllowedTools
		}
	default:
		if utils.IsBlank(cfg["mode"]) {
			cfg["mode"] = "interactive"
		}
	}
}

func buildPlanOnlyPrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "[Plan Mode]\nProduce an execution plan only. Do not modify files."
	}
	return trimmed + "\n\n[Plan Mode]\nProvide a concrete implementation plan only. Do not modify files or run destructive actions."
}
