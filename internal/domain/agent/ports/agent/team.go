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
	Name  string
	Roles []string
}

type teamConfigKey struct{}

// TeamRunRecord captures one dispatched team run for file-based audit/history.
type TeamRunRecord struct {
	RunID         string
	SessionID     string
	ParentRunID   string
	CausationID   string
	TeamName      string
	Goal          string
	DispatchedAt  time.Time
	DispatchState string
	Error         string
	Stages        []TeamRunStageRecord
	Roles         []TeamRunRoleRecord
}

// TeamRunStageRecord captures one stage in a team run.
type TeamRunStageRecord struct {
	Name  string
	Roles []string
}

// TeamRunRoleRecord captures one role assignment in a team run.
type TeamRunRoleRecord struct {
	Name           string
	AgentType      string
	TaskID         string
	DependsOn      []string
	ExecutionMode  string
	AutonomyLevel  string
	WorkspaceMode  string
	InheritContext bool
	Config         map[string]string
	PromptPreview  string
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
