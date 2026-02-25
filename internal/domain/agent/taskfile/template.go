package taskfile

import (
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
	Name  string   `yaml:"name"`
	Roles []string `yaml:"roles"`
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

	// Build stage dependencies.
	deps := buildStageDeps(tmpl.Stages, roleTaskIDs)

	// Create task specs.
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
			Config:         copyStringMap(r.Config),
		}
		tf.Tasks = append(tf.Tasks, spec)
	}

	return tf
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
			Name:  s.Name,
			Roles: s.Roles,
		}
	}
	return TeamTemplate{
		Name:        def.Name,
		Description: def.Description,
		Roles:       roles,
		Stages:      stages,
	}
}

func buildStageDeps(stages []TeamTemplateStage, roleTaskIDs map[string]string) map[string][]string {
	deps := make(map[string][]string)
	for i, stage := range stages {
		if i == 0 {
			for _, roleName := range stage.Roles {
				deps[roleName] = nil
			}
			continue
		}
		prevStage := stages[i-1]
		var prevIDs []string
		for _, prevRole := range prevStage.Roles {
			if taskID, ok := roleTaskIDs[prevRole]; ok {
				prevIDs = append(prevIDs, taskID)
			}
		}
		for _, roleName := range stage.Roles {
			deps[roleName] = append([]string(nil), prevIDs...)
		}
	}
	return deps
}

func renderTeamPrompt(template string, overrides map[string]string, roleName, teamName, goal string) string {
	if override, ok := overrides[roleName]; ok && strings.TrimSpace(override) != "" {
		return override
	}
	if strings.TrimSpace(template) == "" {
		return goal
	}
	result := template
	result = strings.ReplaceAll(result, "{GOAL}", goal)
	result = strings.ReplaceAll(result, "{ROLE}", roleName)
	result = strings.ReplaceAll(result, "{TEAM}", teamName)
	return result
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
