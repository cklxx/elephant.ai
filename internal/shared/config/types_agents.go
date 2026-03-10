package config

import "time"

// ExternalAgentsConfig configures external agent executors.
type ExternalAgentsConfig struct {
	MaxParallelAgents int              `json:"max_parallel_agents" yaml:"max_parallel_agents"`
	ClaudeCode        ClaudeCodeConfig `json:"claude_code" yaml:"claude_code"`
	Codex             CLIAgentConfig   `json:"codex" yaml:"codex"`
	Kimi              CLIAgentConfig   `json:"kimi" yaml:"kimi"`
	Teams             []TeamConfig     `json:"teams" yaml:"teams"`
}

// TeamConfig defines a reusable agent team with role-based collaboration.
type TeamConfig struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Roles       []TeamRoleConfig  `json:"roles" yaml:"roles"`
	Stages      []TeamStageConfig `json:"stages" yaml:"stages"`
}

// TeamRoleConfig defines a single role within a team.
type TeamRoleConfig struct {
	Name              string            `json:"name" yaml:"name"`
	AgentType         string            `json:"agent_type" yaml:"agent_type"`
	CapabilityProfile string            `json:"capability_profile,omitempty" yaml:"capability_profile,omitempty"`
	TargetCLI         string            `json:"target_cli,omitempty" yaml:"target_cli,omitempty"`
	PromptTemplate    string            `json:"prompt_template" yaml:"prompt_template"`
	ExecutionMode     string            `json:"execution_mode" yaml:"execution_mode"`
	AutonomyLevel     string            `json:"autonomy_level" yaml:"autonomy_level"`
	WorkspaceMode     string            `json:"workspace_mode" yaml:"workspace_mode"`
	Config            map[string]string `json:"config" yaml:"config"`
	InheritContext    bool              `json:"inherit_context" yaml:"inherit_context"`
}

// TeamStageConfig defines an execution stage within a team workflow.
// Stages execute in order; within a stage, all role tasks run in parallel.
type TeamStageConfig struct {
	Name       string   `json:"name" yaml:"name"`
	Roles      []string `json:"roles" yaml:"roles"`
	DebateMode bool     `json:"debate_mode,omitempty" yaml:"debate_mode,omitempty"`
}

type ClaudeCodeConfig struct {
	Enabled                bool              `json:"enabled" yaml:"enabled"`
	Binary                 string            `json:"binary" yaml:"binary"`
	DefaultModel           string            `json:"default_model" yaml:"default_model"`
	DefaultMode            string            `json:"default_mode" yaml:"default_mode"`
	AutonomousAllowedTools []string          `json:"autonomous_allowed_tools" yaml:"autonomous_allowed_tools"`
	PlanAllowedTools       []string          `json:"plan_allowed_tools" yaml:"plan_allowed_tools"`
	MaxBudgetUSD           float64           `json:"max_budget_usd" yaml:"max_budget_usd"`
	MaxTurns               int               `json:"max_turns" yaml:"max_turns"`
	Timeout                time.Duration     `json:"timeout" yaml:"timeout"`
	ResumeEnabled          bool              `json:"resume_enabled" yaml:"resume_enabled"`
	Env                    map[string]string `json:"env" yaml:"env"`
}

type CLIAgentConfig struct {
	Enabled            bool              `json:"enabled" yaml:"enabled"`
	Binary             string            `json:"binary" yaml:"binary"`
	DefaultModel       string            `json:"default_model" yaml:"default_model"`
	ApprovalPolicy     string            `json:"approval_policy" yaml:"approval_policy"`
	Sandbox            string            `json:"sandbox" yaml:"sandbox"`
	PlanApprovalPolicy string            `json:"plan_approval_policy" yaml:"plan_approval_policy"`
	PlanSandbox        string            `json:"plan_sandbox" yaml:"plan_sandbox"`
	Timeout            time.Duration     `json:"timeout" yaml:"timeout"`
	ResumeEnabled      bool              `json:"resume_enabled" yaml:"resume_enabled"`
	Env                map[string]string `json:"env" yaml:"env"`
}

type CodexConfig = CLIAgentConfig
type KimiConfig = CLIAgentConfig

// DefaultExternalAgentsConfig provides baseline defaults for external agents.
func DefaultExternalAgentsConfig() ExternalAgentsConfig {
	return ExternalAgentsConfig{
		MaxParallelAgents: 4,
		ClaudeCode: ClaudeCodeConfig{
			Enabled:     false,
			Binary:      "claude",
			DefaultMode: "autonomous",
			MaxTurns:    50,
			Timeout:     30 * time.Minute,
			AutonomousAllowedTools: []string{
				"*",
			},
			PlanAllowedTools: []string{
				"Read",
				"Glob",
				"Grep",
				"WebSearch",
			},
			ResumeEnabled: true,
			Env:           map[string]string{},
		},
		Codex: CLIAgentConfig{
			Enabled:            false,
			Binary:             "codex",
			DefaultModel:       "gpt-5.2-codex",
			ApprovalPolicy:     "never",
			Sandbox:            "danger-full-access",
			PlanApprovalPolicy: "never",
			PlanSandbox:        "read-only",
			Timeout:            30 * time.Minute,
			ResumeEnabled:      true,
			Env:                map[string]string{},
		},
		Kimi: CLIAgentConfig{
			Enabled:            false,
			Binary:             "kimi",
			ApprovalPolicy:     "never",
			Sandbox:            "danger-full-access",
			PlanApprovalPolicy: "never",
			PlanSandbox:        "read-only",
			Timeout:            30 * time.Minute,
			ResumeEnabled:      true,
			Env:                map[string]string{},
		},
	}
}
