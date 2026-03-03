package taskfile

import (
	"maps"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// TeamTemplate is the YAML-serializable form of a team workflow definition.
type TeamTemplate struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description,omitempty"`
	Roles       []TeamTemplateRole  `yaml:"roles"`
	Stages      []TeamTemplateStage `yaml:"stages"`
}

// TeamTemplateRole defines a single role within a team template.
type TeamTemplateRole struct {
	Name           string            `yaml:"name"`
	AgentType      string            `yaml:"agent_type"`
	PromptTemplate string            `yaml:"prompt_template,omitempty"`
	ExecutionMode  string            `yaml:"execution_mode,omitempty"`
	AutonomyLevel  string            `yaml:"autonomy_level,omitempty"`
	WorkspaceMode  string            `yaml:"workspace_mode,omitempty"`
	Config         map[string]string `yaml:"config,omitempty"`
	InheritContext bool              `yaml:"inherit_context,omitempty"`
}

// TeamTemplateStage defines an execution stage within a team workflow.
type TeamTemplateStage struct {
	Name       string   `yaml:"name"`
	Roles      []string `yaml:"roles"`
	DebateMode bool     `yaml:"debate_mode,omitempty"`
}

// RenderTaskFile converts a TeamTemplate + goal into a TaskFile with
// appropriate dependencies between stages.
func RenderTaskFile(tmpl *TeamTemplate, goal string, overrides map[string]string) *TaskFile {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "team-" + tmpl.Name,
		Metadata: map[string]string{
			"team": tmpl.Name,
			"goal": goal,
		},
	}

	// Build role lookup.
	roleByName := make(map[string]TeamTemplateRole, len(tmpl.Roles))
	for _, r := range tmpl.Roles {
		roleByName[r.Name] = r
	}

	// Generate task IDs per role.
	roleTaskIDs := make(map[string]string, len(tmpl.Roles))
	for _, r := range tmpl.Roles {
		roleTaskIDs[r.Name] = "team-" + r.Name
	}

	// Step 1: compute which IDs each stage contributes as outputs for the
	// next stage's dependencies. For debate stages, include both primary and
	// challenger IDs.
	stageOutputIDs := computeStageOutputIDs(tmpl.Stages, roleTaskIDs)

	// Step 2: build primary task dependencies per role using stageOutputIDs.
	deps := buildStageDeps(tmpl.Stages, stageOutputIDs)

	// Step 3: create primary task specs.
	for _, r := range tmpl.Roles {
		prompt := renderTeamPrompt(r.PromptTemplate, overrides, r.Name, tmpl.Name, goal)
		depIDs := deps[r.Name]

		spec := TaskSpec{
			ID:             roleTaskIDs[r.Name],
			Description:    r.Name + " role for team " + tmpl.Name,
			Prompt:         prompt,
			AgentType:      r.AgentType,
			ExecutionMode:  r.ExecutionMode,
			AutonomyLevel:  r.AutonomyLevel,
			WorkspaceMode:  r.WorkspaceMode,
			DependsOn:      depIDs,
			InheritContext: r.InheritContext,
			Config:         maps.Clone(r.Config),
		}
		tf.Tasks = append(tf.Tasks, spec)
	}

	// Step 4: create debate challenger specs for debate stages.
	for _, stage := range tmpl.Stages {
		if !stage.DebateMode {
			continue
		}
		var primaryIDs []string
		for _, roleName := range stage.Roles {
			primaryIDs = append(primaryIDs, roleTaskIDs[roleName])
		}
		for _, roleName := range stage.Roles {
			role := roleByName[roleName]
			debateID := roleTaskIDs[roleName] + "-debate"
			debateSpec := TaskSpec{
				ID:             debateID,
				Description:    roleName + " critical analysis",
				Prompt:         renderDebatePrompt(roleName, tmpl.Name, goal),
				AgentType:      role.AgentType,
				DependsOn:      append([]string(nil), primaryIDs...),
				InheritContext: true,
				WorkspaceMode:  "shared",
			}
			tf.Tasks = append(tf.Tasks, debateSpec)
		}
	}

	return tf
}

// computeStageOutputIDs returns, for each stage index, the task IDs that the
// following stage should depend on. For debate stages this includes both the
// primary role IDs and their challenger IDs.
func computeStageOutputIDs(stages []TeamTemplateStage, roleTaskIDs map[string]string) [][]string {
	out := make([][]string, len(stages))
	for i, stage := range stages {
		var ids []string
		for _, roleName := range stage.Roles {
			if taskID, ok := roleTaskIDs[roleName]; ok {
				ids = append(ids, taskID)
			}
		}
		if stage.DebateMode {
			for _, roleName := range stage.Roles {
				if taskID, ok := roleTaskIDs[roleName]; ok {
					ids = append(ids, taskID+"-debate")
				}
			}
		}
		out[i] = ids
	}
	return out
}

// buildStageDeps returns, for each role name, the list of task IDs it must
// wait for. stageOutputIDs[i] is the full set of IDs produced by stage i
// (including any debate challengers).
func buildStageDeps(stages []TeamTemplateStage, stageOutputIDs [][]string) map[string][]string {
	deps := make(map[string][]string)
	for i, stage := range stages {
		if i == 0 {
			for _, roleName := range stage.Roles {
				deps[roleName] = nil
			}
			continue
		}
		prevIDs := stageOutputIDs[i-1]
		for _, roleName := range stage.Roles {
			deps[roleName] = append([]string(nil), prevIDs...)
		}
	}
	return deps
}

// TeamTemplateFromDefinition converts a domain TeamDefinition into a
// YAML-serializable TeamTemplate.
func TeamTemplateFromDefinition(def agent.TeamDefinition) TeamTemplate {
	roles := make([]TeamTemplateRole, len(def.Roles))
	for i, r := range def.Roles {
		roles[i] = TeamTemplateRole{
			Name:           r.Name,
			AgentType:      r.AgentType,
			PromptTemplate: r.PromptTemplate,
			ExecutionMode:  r.ExecutionMode,
			AutonomyLevel:  r.AutonomyLevel,
			WorkspaceMode:  r.WorkspaceMode,
			Config:         r.Config,
			InheritContext: r.InheritContext,
		}
	}
	stages := make([]TeamTemplateStage, len(def.Stages))
	for i, s := range def.Stages {
		stages[i] = TeamTemplateStage{
			Name:       s.Name,
			Roles:      s.Roles,
			DebateMode: s.DebateMode,
		}
	}
	return TeamTemplate{
		Name:        def.Name,
		Description: def.Description,
		Roles:       roles,
		Stages:      stages,
	}
}

func renderTeamPrompt(template string, overrides map[string]string, roleName, teamName, goal string) string {
	if override, ok := overrides[roleName]; ok && strings.TrimSpace(override) != "" {
		return override
	}
	if strings.TrimSpace(template) == "" {
		return goal
	}
	return strings.NewReplacer(
		"{GOAL}", goal,
		"{ROLE}", roleName,
		"{TEAM}", teamName,
	).Replace(template)
}

// renderDebatePrompt builds the critical-analysis prompt for a debate challenger.
func renderDebatePrompt(roleName, teamName, goal string) string {
	var sb strings.Builder
	sb.WriteString("[DEBATE MODE — Critical Analysis for team " + teamName + "]\n\n")
	sb.WriteString("Your team independently analyzed: " + goal + "\n")
	sb.WriteString("Their conclusions appear above (in the collaboration context).\n\n")
	sb.WriteString("Your role as critic (" + roleName + "):\n")
	sb.WriteString("1. Identify assumptions that might be wrong in each conclusion\n")
	sb.WriteString("2. Find specific counterexamples or edge cases that were missed\n")
	sb.WriteString("3. Rate each conclusion's defensibility (low/medium/high) with one-line justification\n")
	sb.WriteString("4. State which conclusion you find most credible after analysis, and why\n\n")
	sb.WriteString("Be precise: reference specific claims, not vague criticism.\n")
	return sb.String()
}

