package taskfile

import (
	"maps"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// RenderTaskFile converts a TeamDefinition + goal into a TaskFile with
// appropriate dependencies between stages.
func RenderTaskFile(def *agent.TeamDefinition, goal string, overrides map[string]string) *TaskFile {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "team-" + def.Name,
		Metadata: map[string]string{
			"team": def.Name,
			"goal": goal,
		},
	}

	// Build role lookup.
	roleByName := make(map[string]agent.TeamRoleDefinition, len(def.Roles))
	for _, r := range def.Roles {
		roleByName[r.Name] = r
	}

	// Generate task IDs per role.
	roleTaskIDs := make(map[string]string, len(def.Roles))
	for _, r := range def.Roles {
		roleTaskIDs[r.Name] = "team-" + r.Name
	}

	// Step 1: compute which IDs each stage contributes as outputs for the
	// next stage's dependencies. For debate stages, include both primary and
	// challenger IDs.
	stageOutputIDs := computeStageOutputIDs(def.Stages, roleTaskIDs)

	// Step 2: build primary task dependencies per role using stageOutputIDs.
	deps := buildStageDeps(def.Stages, stageOutputIDs)

	// Step 3: create primary task specs.
	for _, r := range def.Roles {
		prompt := renderTeamPrompt(r.PromptTemplate, overrides, r.Name, def.Name, goal)
		depIDs := deps[r.Name]

		spec := TaskSpec{
			ID:             roleTaskIDs[r.Name],
			Description:    r.Name + " role for team " + def.Name,
			Prompt:         prompt,
			AgentType:      r.AgentType,
			ExecutionMode:  r.ExecutionMode,
			AutonomyLevel:  r.AutonomyLevel,
			WorkspaceMode:  r.WorkspaceMode,
			DependsOn:      depIDs,
			InheritContext: r.InheritContext,
			Config:         maps.Clone(r.Config),
		}
		if spec.Config == nil {
			spec.Config = make(map[string]string)
		}
		if strings.TrimSpace(r.CapabilityProfile) != "" {
			spec.Config["capability_profile"] = strings.TrimSpace(r.CapabilityProfile)
		}
		if strings.TrimSpace(r.TargetCLI) != "" {
			spec.Config["target_cli"] = strings.TrimSpace(r.TargetCLI)
		}
		tf.Tasks = append(tf.Tasks, spec)
	}

	// Step 4: create debate challenger specs for debate stages.
	for _, stage := range def.Stages {
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
				Prompt:         renderDebatePrompt(roleName, def.Name, goal),
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
func computeStageOutputIDs(stages []agent.TeamStageDefinition, roleTaskIDs map[string]string) [][]string {
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
func buildStageDeps(stages []agent.TeamStageDefinition, stageOutputIDs [][]string) map[string][]string {
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
