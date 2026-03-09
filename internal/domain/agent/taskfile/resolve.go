package taskfile

import (
	"strconv"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// resolveDefaults merges TaskFile-level defaults into each TaskSpec,
// applying task-level values over defaults. Returns a new slice; the
// original TaskFile is not modified.
func resolveDefaults(tf *TaskFile) []TaskSpec {
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

// applyCodingDefaults sets sensible defaults for external coding agents.
// All coding-specific knobs (verify, merge_on_success, retry_max_attempts, etc.)
// live in the Config map — no dedicated struct fields.
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

	if !agent.IsCodingExternalAgent(t.AgentType) {
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
		if isPlan {
			t.Config["verify"] = "false"
		} else {
			t.Config["verify"] = "true"
		}
	}

	if t.Config["merge_on_success"] == "" {
		if isPlan {
			t.Config["merge_on_success"] = "false"
		} else {
			t.Config["merge_on_success"] = "true"
		}
	}

	if t.Config["retry_max_attempts"] == "" {
		if isPlan {
			t.Config["retry_max_attempts"] = "1"
		} else {
			t.Config["retry_max_attempts"] = "3"
		}
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
	t.Config["workspace_mode"] = t.WorkspaceMode
}

// specToDispatchRequest converts a resolved TaskSpec into a BackgroundDispatchRequest.
func specToDispatchRequest(spec TaskSpec, causationID string) agent.BackgroundDispatchRequest {
	prompt := spec.Prompt
	if spec.ContextPreamble != "" {
		prompt = spec.ContextPreamble + "\n\n---\n\n" + prompt
	}

	// Flatten RuntimeMeta into Config for bridge consumption.
	flattenRuntimeMeta(spec.RuntimeMeta, spec.Config)

	// Override AgentType from SelectedAgentType if set.
	agentType := spec.AgentType
	if strings.TrimSpace(spec.RuntimeMeta.SelectedAgentType) != "" {
		agentType = strings.TrimSpace(spec.RuntimeMeta.SelectedAgentType)
	}

	return agent.BackgroundDispatchRequest{
		TaskID:         spec.ID,
		Description:    spec.Description,
		Prompt:         prompt,
		AgentType:      agent.CanonicalAgentType(agentType),
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

// setIfPresent sets cfg[key] = val when val is non-empty after trimming.
func setIfPresent(cfg map[string]string, key, val string) {
	if v := strings.TrimSpace(val); v != "" {
		cfg[key] = v
	}
}

// flattenRuntimeMeta writes all non-empty TeamRuntimeMeta fields into the
// Config map so the bridge subprocess can consume them as flat key-value pairs.
func flattenRuntimeMeta(meta TeamRuntimeMeta, cfg map[string]string) {
	if cfg == nil {
		return
	}
	setIfPresent(cfg, "team_id", meta.TeamID)
	setIfPresent(cfg, "role_id", meta.RoleID)
	setIfPresent(cfg, "team_runtime_dir", meta.TeamRuntimeDir)
	setIfPresent(cfg, "team_event_log", meta.TeamEventLog)
	setIfPresent(cfg, "capability_profile", meta.CapabilityProfile)
	setIfPresent(cfg, "target_cli", meta.TargetCLI)
	setIfPresent(cfg, "selected_cli", meta.SelectedCLI)
	if len(meta.FallbackCLIs) > 0 {
		cfg["fallback_clis"] = strings.Join(meta.FallbackCLIs, ",")
	}
	setIfPresent(cfg, "binary", meta.Binary)
	setIfPresent(cfg, "role_log_path", meta.RoleLogPath)
	setIfPresent(cfg, "tmux_session", meta.TmuxSession)
	setIfPresent(cfg, "tmux_pane", meta.TmuxPane)
	setIfPresent(cfg, "selected_agent_type", meta.SelectedAgentType)
}

