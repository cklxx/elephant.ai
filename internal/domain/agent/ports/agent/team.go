package agent

import (
	"context"
	"time"
)

// TeamDefinition describes a reusable agent team for the tool layer.
// Defined in the domain to avoid import cycles with config package.
type TeamDefinition struct {
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
	Roles       []TeamRoleDefinition  `yaml:"roles"`
	Stages      []TeamStageDefinition `yaml:"stages"`
}

// TeamRoleDefinition defines a single role within a team.
type TeamRoleDefinition struct {
	Name              string            `yaml:"name"`
	AgentType         string            `yaml:"agent_type"`
	CapabilityProfile string            `yaml:"capability_profile,omitempty"`
	TargetCLI         string            `yaml:"target_cli,omitempty"`
	PromptTemplate    string            `yaml:"prompt_template,omitempty"`
	ExecutionMode     string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel     string            `yaml:"autonomy_level,omitempty"`
	WorkspaceMode     string            `yaml:"workspace_mode,omitempty"`
	Config            map[string]string `yaml:"config,omitempty"`
	InheritContext    bool              `yaml:"inherit_context,omitempty"`
}

// TeamStageDefinition defines an execution stage within a team workflow.
type TeamStageDefinition struct {
	Name       string   `yaml:"name"`
	Roles      []string `yaml:"roles"`
	DebateMode bool     `yaml:"debate_mode,omitempty"`
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
	Name              string            `yaml:"name"`
	AgentType         string            `yaml:"agent_type"`
	CapabilityProfile string            `yaml:"capability_profile,omitempty"`
	TargetCLI         string            `yaml:"target_cli,omitempty"`
	SelectedCLI       string            `yaml:"selected_cli,omitempty"`
	FallbackCLIs      []string          `yaml:"fallback_clis,omitempty"`
	TaskID            string            `yaml:"task_id"`
	DependsOn         []string          `yaml:"depends_on,omitempty"`
	ExecutionMode     string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel     string            `yaml:"autonomy_level,omitempty"`
	WorkspaceMode     string            `yaml:"workspace_mode,omitempty"`
	InheritContext    bool              `yaml:"inherit_context,omitempty"`
	Config            map[string]string `yaml:"config,omitempty"`
	PromptPreview     string            `yaml:"prompt_preview,omitempty"`
}

// TeamRunRecorder persists team run records to a durable store (typically file-based).
type TeamRunRecorder interface {
	RecordTeamRun(ctx context.Context, record TeamRunRecord) (string, error)
}

type teamRunRecorderKey struct{}

// OrchestrationContext bundles the three values that the ReAct runtime injects
// into context for orchestration tools: team definitions, a run recorder, and
// a background task dispatcher.  Storing them as a single context value
// reduces context.WithValue overhead from three calls to one.
type OrchestrationContext struct {
	TeamDefinitions []TeamDefinition
	TeamRunRecorder TeamRunRecorder
	Dispatcher      BackgroundTaskDispatcher
}

type orchestrationContextKey struct{}

// WithOrchestrationContext stores an OrchestrationContext in ctx.
func WithOrchestrationContext(ctx context.Context, oc OrchestrationContext) context.Context {
	return context.WithValue(ctx, orchestrationContextKey{}, oc)
}

// GetOrchestrationContext retrieves the OrchestrationContext from ctx.
// Returns the zero value when none is present.
func GetOrchestrationContext(ctx context.Context) OrchestrationContext {
	if ctx == nil {
		return OrchestrationContext{}
	}
	oc, _ := ctx.Value(orchestrationContextKey{}).(OrchestrationContext)
	return oc
}

// WithTeamDefinitions stores team definitions in context for tool access.
func WithTeamDefinitions(ctx context.Context, teams []TeamDefinition) context.Context {
	return context.WithValue(ctx, teamConfigKey{}, teams)
}

// GetTeamDefinitions retrieves team definitions from context.
// Falls back to OrchestrationContext when no standalone value is found.
func GetTeamDefinitions(ctx context.Context) []TeamDefinition {
	if ctx == nil {
		return nil
	}
	if teams, ok := ctx.Value(teamConfigKey{}).([]TeamDefinition); ok {
		return teams
	}
	return GetOrchestrationContext(ctx).TeamDefinitions
}

// WithTeamRunRecorder stores a TeamRunRecorder in context for run_tasks.
func WithTeamRunRecorder(ctx context.Context, recorder TeamRunRecorder) context.Context {
	return context.WithValue(ctx, teamRunRecorderKey{}, recorder)
}

// GetTeamRunRecorder retrieves the TeamRunRecorder from context.
// Falls back to OrchestrationContext when no standalone value is found.
func GetTeamRunRecorder(ctx context.Context) TeamRunRecorder {
	if ctx == nil {
		return nil
	}
	if recorder, ok := ctx.Value(teamRunRecorderKey{}).(TeamRunRecorder); ok {
		return recorder
	}
	return GetOrchestrationContext(ctx).TeamRunRecorder
}
