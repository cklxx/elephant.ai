package taskfile

import (
	"strconv"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// ResolveDefaults merges TaskFile-level defaults into each TaskSpec,
// applying task-level values over defaults. Returns a new slice; the
// original TaskFile is not modified.
func ResolveDefaults(tf *TaskFile) []TaskSpec {
	out := make([]TaskSpec, len(tf.Tasks))
	for i, t := range tf.Tasks {
		out[i] = mergeDefaults(t, tf.Defaults)
		applyCodingDefaults(&out[i])
	}
	return out
}

func mergeDefaults(t TaskSpec, d TaskDefaults) TaskSpec {
	if t.AgentType == "" {
		t.AgentType = d.AgentType
	}
	if t.ExecutionMode == "" {
		t.ExecutionMode = d.ExecutionMode
	}
	if t.AutonomyLevel == "" {
		t.AutonomyLevel = d.AutonomyLevel
	}
	if t.WorkspaceMode == "" {
		t.WorkspaceMode = d.WorkspaceMode
	}
	if t.ContextPreamble == "" {
		t.ContextPreamble = d.ContextPreamble
	}
	if t.MaxBudget == 0 {
		t.MaxBudget = d.MaxBudget
	}
	if len(d.Config) > 0 {
		merged := make(map[string]string, len(d.Config)+len(t.Config))
		for k, v := range d.Config {
			merged[k] = v
		}
		for k, v := range t.Config {
			merged[k] = v
		}
		t.Config = merged
	}
	return t
}

// applyCodingDefaults sets sensible defaults for external coding agents,
// mirroring the legacy coding defaults logic.
func applyCodingDefaults(t *TaskSpec) {
	if t.Config == nil {
		t.Config = make(map[string]string)
	}

	if t.ExecutionMode != "" {
		t.Config["execution_mode"] = t.ExecutionMode
	}
	if t.AutonomyLevel != "" {
		t.Config["autonomy_level"] = t.AutonomyLevel
	}

	if !isCodingExternalAgent(t.AgentType) {
		return
	}

	if t.Config["task_kind"] == "" {
		t.Config["task_kind"] = "coding"
	}

	isPlan := t.ExecutionMode == "plan"

	if t.AutonomyLevel == "controlled" {
		t.AutonomyLevel = "full"
		t.Config["autonomy_level"] = "full"
	}

	if t.Config["verify"] == "" {
		if t.Verify != nil {
			t.Config["verify"] = strconv.FormatBool(*t.Verify)
		} else if isPlan {
			t.Config["verify"] = "false"
		} else {
			t.Config["verify"] = "true"
		}
	}

	if t.Config["merge_on_success"] == "" {
		if t.MergeOnSuccess != nil {
			t.Config["merge_on_success"] = strconv.FormatBool(*t.MergeOnSuccess)
		} else if isPlan {
			t.Config["merge_on_success"] = "false"
		} else {
			t.Config["merge_on_success"] = "true"
		}
	}

	if t.Config["coding_profile"] == "" && t.CodingProfile != "" {
		t.Config["coding_profile"] = t.CodingProfile
	}

	if t.RetryMax != nil {
		t.Config["retry_max_attempts"] = strconv.Itoa(*t.RetryMax)
	} else if t.Config["retry_max_attempts"] == "" {
		if isPlan {
			t.Config["retry_max_attempts"] = "1"
		} else {
			t.Config["retry_max_attempts"] = "3"
		}
	}

	if t.VerifyBuildCmd != "" {
		t.Config["verify_build_cmd"] = t.VerifyBuildCmd
	}
	if t.VerifyTestCmd != "" {
		t.Config["verify_test_cmd"] = t.VerifyTestCmd
	}
	if t.VerifyLintCmd != "" {
		t.Config["verify_lint_cmd"] = t.VerifyLintCmd
	}

	if t.MergeStrategy != "" {
		t.Config["merge_strategy"] = t.MergeStrategy
	}

	if t.MaxBudget > 0 && t.Config["max_budget_usd"] == "" {
		t.Config["max_budget_usd"] = strconv.FormatFloat(t.MaxBudget, 'f', -1, 64)
	}

	// Workspace mode defaults.
	if t.WorkspaceMode == "" {
		if t.Config["task_kind"] == "coding" && t.ExecutionMode != "plan" {
			t.WorkspaceMode = string(agent.WorkspaceModeWorktree)
		} else {
			t.WorkspaceMode = string(agent.WorkspaceModeShared)
		}
	}
}

// SpecToDispatchRequest converts a resolved TaskSpec into a BackgroundDispatchRequest.
func SpecToDispatchRequest(spec TaskSpec, causationID string) agent.BackgroundDispatchRequest {
	prompt := spec.Prompt
	if spec.ContextPreamble != "" {
		prompt = spec.ContextPreamble + "\n\n---\n\n" + prompt
	}
	return agent.BackgroundDispatchRequest{
		TaskID:         spec.ID,
		Description:    spec.Description,
		Prompt:         prompt,
		AgentType:      canonicalAgentType(spec.AgentType),
		ExecutionMode:  spec.ExecutionMode,
		AutonomyLevel:  spec.AutonomyLevel,
		CausationID:    causationID,
		Config:         spec.Config,
		DependsOn:      spec.DependsOn,
		WorkspaceMode:  agent.WorkspaceMode(spec.WorkspaceMode),
		FileScope:      spec.FileScope,
		InheritContext: spec.InheritContext,
	}
}

func canonicalAgentType(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch strings.ToLower(trimmed) {
	case "":
		return ""
	case "internal":
		return "internal"
	case "codex":
		return "codex"
	case "kimi", "kimi_cli", "kimi-cli", "k2", "kimi cli":
		return "kimi"
	case "claude_code", "claude-code", "claude code":
		return "claude_code"
	default:
		return trimmed
	}
}

func isCodingExternalAgent(agentType string) bool {
	switch canonicalAgentType(agentType) {
	case "codex", "claude_code", "kimi":
		return true
	default:
		return false
	}
}