package agent

import (
	"context"
	"time"
)

// TeamDefinition describes a reusable agent team for the tool layer.
// Defined in the domain to avoid import cycles with config package.
type TeamDefinition struct {
	Name        string
	Description string
	Roles       []TeamRoleDefinition
	Stages      []TeamStageDefinition
}

// TeamRoleDefinition defines a single role within a team.
type TeamRoleDefinition struct {
	Name           string
	AgentType      string
	PromptTemplate string
	ExecutionMode  string
	AutonomyLevel  string
	WorkspaceMode  string
	Config         map[string]string
	InheritContext bool
}

// TeamStageDefinition defines an execution stage within a team workflow.
type TeamStageDefinition struct {
	Name       string
	Roles      []string
	DebateMode bool
}

type teamConfigKey struct{}

// TeamRunRecord captures one dispatched team run for file-based audit/history.
type TeamRunRecord struct {
	RunID         string               `yaml:"run_id"`
	SessionID     string               `yaml:"session_id,omitempty"`
	ParentRunID   string               `yaml:"parent_run_id,omitempty"`
	CausationID   string               `yaml:"causation_id,omitempty"`
	TeamName      string               `yaml:"team_name"`
	Goal          string               `yaml:"goal"`
	DispatchedAt  time.Time            `yaml:"dispatched_at"`
	DispatchState string               `yaml:"dispatch_state"`
	Error         string               `yaml:"error,omitempty"`
	Stages        []TeamRunStageRecord `yaml:"stages,omitempty"`
	Roles         []TeamRunRoleRecord  `yaml:"roles,omitempty"`
}

// TeamRunStageRecord captures one stage in a team run.
type TeamRunStageRecord struct {
	Name  string   `yaml:"name"`
	Roles []string `yaml:"roles"`
}

// TeamRunRoleRecord captures one role assignment in a team run.
type TeamRunRoleRecord struct {
	Name           string            `yaml:"name"`
	AgentType      string            `yaml:"agent_type"`
	TaskID         string            `yaml:"task_id"`
	DependsOn      []string          `yaml:"depends_on,omitempty"`
	ExecutionMode  string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel  string            `yaml:"autonomy_level,omitempty"`
	WorkspaceMode  string            `yaml:"workspace_mode,omitempty"`
	InheritContext bool              `yaml:"inherit_context,omitempty"`
	Config         map[string]string `yaml:"config,omitempty"`
	PromptPreview  string            `yaml:"prompt_preview,omitempty"`
}

// TeamRunRecorder persists team run records to a durable store (typically file-based).
type TeamRunRecorder interface {
	RecordTeamRun(ctx context.Context, record TeamRunRecord) (string, error)
}

type teamRunRecorderKey struct{}

// WithTeamDefinitions stores team definitions in context for tool access.
func WithTeamDefinitions(ctx context.Context, teams []TeamDefinition) context.Context {
	return context.WithValue(ctx, teamConfigKey{}, teams)
}

// GetTeamDefinitions retrieves team definitions from context.
func GetTeamDefinitions(ctx context.Context) []TeamDefinition {
	if ctx == nil {
		return nil
	}
	teams, _ := ctx.Value(teamConfigKey{}).([]TeamDefinition)
	return teams
}

// WithTeamRunRecorder stores a TeamRunRecorder in context for run_tasks.
func WithTeamRunRecorder(ctx context.Context, recorder TeamRunRecorder) context.Context {
	return context.WithValue(ctx, teamRunRecorderKey{}, recorder)
}

// GetTeamRunRecorder retrieves the TeamRunRecorder from context.
func GetTeamRunRecorder(ctx context.Context) TeamRunRecorder {
	if ctx == nil {
		return nil
	}
	recorder, _ := ctx.Value(teamRunRecorderKey{}).(TeamRunRecorder)
	return recorder
}
