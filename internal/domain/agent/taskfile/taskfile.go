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
	AgentType     string            `yaml:"agent_type,omitempty"`
	ExecutionMode string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel string            `yaml:"autonomy_level,omitempty"`
	WorkspaceMode string            `yaml:"workspace_mode,omitempty"`
	Config        map[string]string `yaml:"config,omitempty"`
}

// TaskSpec defines a single task within a TaskFile.
type TaskSpec struct {
	ID             string            `yaml:"id"`
	Description    string            `yaml:"description"`
	Prompt         string            `yaml:"prompt"`
	AgentType      string            `yaml:"agent_type,omitempty"`
	ExecutionMode  string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel  string            `yaml:"autonomy_level,omitempty"`
	DependsOn      []string          `yaml:"depends_on,omitempty"`
	WorkspaceMode  string            `yaml:"workspace_mode,omitempty"`
	FileScope      []string          `yaml:"file_scope,omitempty"`
	InheritContext bool              `yaml:"inherit_context,omitempty"`
	Config         map[string]string `yaml:"config,omitempty"`
	Verify         *bool             `yaml:"verify,omitempty"`
	MergeOnSuccess *bool             `yaml:"merge_on_success,omitempty"`
	MergeStrategy  string            `yaml:"merge_strategy,omitempty"`
	CodingProfile  string            `yaml:"coding_profile,omitempty"`
	RetryMax       *int              `yaml:"retry_max,omitempty"`
	VerifyBuildCmd string            `yaml:"verify_build_cmd,omitempty"`
	VerifyTestCmd  string            `yaml:"verify_test_cmd,omitempty"`
	VerifyLintCmd  string            `yaml:"verify_lint_cmd,omitempty"`
}
