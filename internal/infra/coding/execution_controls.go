package coding

import "strings"

const (
	executionModeExecute = "execute"
	executionModePlan    = "plan"

	autonomyControlled = "controlled"
	autonomySemi       = "semi"
	autonomyFull       = "full"
)

const (
	claudePlanAllowedTools = "Read,Glob,Grep,WebSearch"
	claudeSemiAllowedTools = "Read,Glob,Grep,WebSearch,Write,Edit,Bash"
)

func normalizeExecutionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case executionModePlan:
		return executionModePlan
	default:
		return executionModeExecute
	}
}

func normalizeAutonomyLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case autonomyFull:
		return autonomyFull
	case autonomySemi:
		return autonomySemi
	default:
		return autonomyControlled
	}
}

// applyExecutionControls normalizes cross-agent controls into agent-specific config keys.
func applyExecutionControls(agentType, mode, level string, config map[string]string) map[string]string {
	cfg := cloneStringMap(config)
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

	normalizedMode := normalizeExecutionMode(modeRaw)
	normalizedLevel := normalizeAutonomyLevel(levelRaw)
	cfg["execution_mode"] = normalizedMode
	cfg["autonomy_level"] = normalizedLevel

	switch strings.ToLower(strings.TrimSpace(agentType)) {
	case "codex":
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
		if strings.TrimSpace(cfg["sandbox"]) == "" {
			cfg["sandbox"] = "danger-full-access"
		}
		if strings.TrimSpace(cfg["approval_policy"]) == "" {
			cfg["approval_policy"] = "never"
		}
	case autonomySemi:
		if strings.TrimSpace(cfg["sandbox"]) == "" {
			cfg["sandbox"] = "workspace-write"
		}
		if strings.TrimSpace(cfg["approval_policy"]) == "" {
			cfg["approval_policy"] = "on-failure"
		}
	default:
		if strings.TrimSpace(cfg["sandbox"]) == "" {
			cfg["sandbox"] = "workspace-write"
		}
		if strings.TrimSpace(cfg["approval_policy"]) == "" {
			cfg["approval_policy"] = "on-request"
		}
	}
}

func applyClaudeExecutionControls(cfg map[string]string, mode, level string) {
	if mode == executionModePlan {
		cfg["mode"] = "autonomous"
		if strings.TrimSpace(cfg["allowed_tools"]) == "" {
			cfg["allowed_tools"] = claudePlanAllowedTools
		}
		return
	}

	switch level {
	case autonomyFull:
		if strings.TrimSpace(cfg["mode"]) == "" {
			cfg["mode"] = "autonomous"
		}
		if strings.TrimSpace(cfg["allowed_tools"]) == "" {
			cfg["allowed_tools"] = "*"
		}
	case autonomySemi:
		if strings.TrimSpace(cfg["mode"]) == "" {
			cfg["mode"] = "autonomous"
		}
		if strings.TrimSpace(cfg["allowed_tools"]) == "" {
			cfg["allowed_tools"] = claudeSemiAllowedTools
		}
	default:
		if strings.TrimSpace(cfg["mode"]) == "" {
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
