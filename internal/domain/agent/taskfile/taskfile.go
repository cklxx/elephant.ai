// Package taskfile defines the YAML-based task file format for orchestration.
// A TaskFile describes a set of tasks with dependencies that can be dispatched
// to the BackgroundTaskManager via the run_tasks tool.
package taskfile

// TaskFile is the top-level YAML structure for file-based orchestration.
type TaskFile struct {
	Version  string            `yaml:"version"`
	PlanID   string            `yaml:"plan_id"`
	Defaults TaskDefaults      `yaml:"defaults,omitempty"`
	Tasks    []TaskSpec        `yaml:"tasks"`
	Metadata map[string]string `yaml:"metadata,omitempty"`
}

// TaskDefaults provides default values applied to all tasks in the file.
type TaskDefaults struct {
	AgentType       string            `yaml:"agent_type,omitempty"`
	ExecutionMode   string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel   string            `yaml:"autonomy_level,omitempty"`
	WorkspaceMode   string            `yaml:"workspace_mode,omitempty"`
	Config          map[string]string `yaml:"config,omitempty"`
	ContextPreamble string            `yaml:"context_preamble,omitempty"`
	MaxBudget       float64           `yaml:"max_budget_usd,omitempty"`
}

// TaskSpec defines a single task within a TaskFile.
// Coding-specific settings (verify, merge_on_success, retry_max_attempts, etc.)
// are set via the Config map — see applyCodingDefaults in resolve.go.
type TaskSpec struct {
	ID              string            `yaml:"id"`
	Description     string            `yaml:"description"`
	Prompt          string            `yaml:"prompt"`
	AgentType       string            `yaml:"agent_type,omitempty"`
	ExecutionMode   string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel   string            `yaml:"autonomy_level,omitempty"`
	DependsOn       []string          `yaml:"depends_on,omitempty"`
	WorkspaceMode   string            `yaml:"workspace_mode,omitempty"`
	FileScope       []string          `yaml:"file_scope,omitempty"`
	InheritContext  bool              `yaml:"inherit_context,omitempty"`
	Config          map[string]string `yaml:"config,omitempty"`
	ContextPreamble string            `yaml:"context_preamble,omitempty"`
	MaxBudget       float64           `yaml:"max_budget_usd,omitempty"`
	RuntimeMeta     TeamRuntimeMeta   `yaml:"-"`
}

// TeamRuntimeMeta captures team bootstrap runtime bindings for a task.
// Populated by applyBootstrapToTaskFile; flattened into Config for bridge
// consumption via flattenRuntimeMeta.
type TeamRuntimeMeta struct {
	TeamID            string   `yaml:"team_id,omitempty"`
	RoleID            string   `yaml:"role_id,omitempty"`
	TeamRuntimeDir    string   `yaml:"team_runtime_dir,omitempty"`
	TeamEventLog      string   `yaml:"team_event_log,omitempty"`
	CapabilityProfile string   `yaml:"capability_profile,omitempty"`
	TargetCLI         string   `yaml:"target_cli,omitempty"`
	SelectedCLI       string   `yaml:"selected_cli,omitempty"`
	FallbackCLIs      []string `yaml:"fallback_clis,omitempty"`
	Binary            string   `yaml:"binary,omitempty"`
	RoleLogPath       string   `yaml:"role_log_path,omitempty"`
	TmuxSession       string   `yaml:"tmux_session,omitempty"`
	TmuxPane          string   `yaml:"tmux_pane,omitempty"`
	SelectedAgentType string   `yaml:"selected_agent_type,omitempty"`
}
