package agent

import "context"

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
