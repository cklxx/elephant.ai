package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/executioncontrol"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils/id"
	"alex/internal/shared/utils"
)

type teamDispatchRolePlan struct {
	role       agent.TeamRoleDefinition
	prompt     string
	taskConfig map[string]string
}

type teamDispatch struct {
	shared.BaseTool
}

// NewTeamDispatch creates the team_dispatch tool for agent team workflows.
func NewTeamDispatch() *teamDispatch {
	return &teamDispatch{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "team_dispatch",
				Description: `Dispatch an agent team to collaboratively execute a goal. Teams are pre-configured workflows where different agents handle different roles (e.g., Codex executes code, Claude Code summarizes results). The team follows a staged DAG: each stage completes before the next starts. Use bg_status to monitor progress and bg_collect to retrieve results. Pass team="list" to see available teams.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"team": {
							Type:        "string",
							Description: `Team name from config (e.g., "execute_and_report"). Pass "list" to see available teams.`,
						},
						"goal": {
							Type:        "string",
							Description: "The overall goal for the team to accomplish.",
						},
						"prompts": {
							Type:        "object",
							Description: "Optional per-role prompt overrides (role_name -> prompt). Replaces the role's template entirely.",
						},
						"config": {
							Type:        "object",
							Description: "Optional config overrides applied to all tasks.",
						},
					},
					Required: []string{"team", "goal"},
				},
			},
			ports.ToolMetadata{
				Name:     "team_dispatch",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"team", "orchestration", "collaboration"},
			},
		),
	}
}

func (t *teamDispatch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "team", "goal", "prompts", "config":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	teamName, errResult := shared.RequireStringArg(call.Arguments, call.ID, "team")
	if errResult != nil {
		return errResult, nil
	}

	teams := agent.GetTeamDefinitions(ctx)

	// Handle "list" command.
	if strings.EqualFold(strings.TrimSpace(teamName), "list") {
		return t.listTeams(call.ID, teams)
	}

	goal, errResult := shared.RequireStringArg(call.Arguments, call.ID, "goal")
	if errResult != nil {
		return errResult, nil
	}

	promptOverrides, err := parseStringMap(call.Arguments, "prompts")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	configOverrides, err := parseStringMap(call.Arguments, "config")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	// Look up team definition.
	team := findTeam(teams, teamName)
	if team == nil {
		return shared.ToolError(call.ID, "unknown team %q; use team=\"list\" to see available teams", teamName)
	}

	if err := validateTeam(team); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}

	// Build role → taskID mapping and dispatch tasks.
	roleTaskIDs := make(map[string]string, len(team.Roles))
	for _, role := range team.Roles {
		roleTaskIDs[role.Name] = "team-" + role.Name + "-" + id.NewKSUID()
	}

	// Build stage ordering: stage N+1 depends on all stage N task IDs.
	stageDeps := buildStageDeps(team.Stages, roleTaskIDs)
	rolePlans := make(map[string]teamDispatchRolePlan, len(team.Roles))
	roleRecords := make([]agent.TeamRunRoleRecord, 0, len(team.Roles))
	for _, stage := range team.Stages {
		for _, roleName := range stage.Roles {
			role := findRole(team.Roles, roleName)
			if role == nil {
				return shared.ToolError(call.ID, "stage %q references unknown role %q", stage.Name, roleName)
			}

			prompt := renderTeamPrompt(role.PromptTemplate, promptOverrides, roleName, teamName, goal)
			taskConfig := buildTeamTaskConfig(role, configOverrides)
			rolePlans[roleName] = teamDispatchRolePlan{
				role:       *role,
				prompt:     prompt,
				taskConfig: taskConfig,
			}

			roleRecords = append(roleRecords, agent.TeamRunRoleRecord{
				Name:           role.Name,
				AgentType:      canonicalAgentType(role.AgentType),
				TaskID:         roleTaskIDs[roleName],
				DependsOn:      append([]string(nil), stageDeps[roleName]...),
				ExecutionMode:  normalizeExecutionMode(role.ExecutionMode),
				AutonomyLevel:  normalizeAutonomy(role.AutonomyLevel),
				WorkspaceMode:  role.WorkspaceMode,
				InheritContext: role.InheritContext,
				Config:         ports.CloneStringMap(taskConfig),
				PromptPreview:  truncateGoal(prompt, 200),
			})
		}
	}
	recorder := agent.GetTeamRunRecorder(ctx)
	teamRunRecord := agent.TeamRunRecord{
		RunID:         "teamrun-" + id.NewKSUID(),
		SessionID:     id.SessionIDFromContext(ctx),
		ParentRunID:   firstNonEmpty(id.RunIDFromContext(ctx), id.ParentRunIDFromContext(ctx)),
		CausationID:   call.ID,
		TeamName:      teamName,
		Goal:          goal,
		DispatchedAt:  time.Now().UTC(),
		DispatchState: "dispatched",
		Stages:        buildTeamRunStageRecords(team.Stages),
		Roles:         roleRecords,
	}

	var dispatchedIDs []string
	for _, stage := range team.Stages {
		for _, roleName := range stage.Roles {
			plan, ok := rolePlans[roleName]
			if !ok {
				return shared.ToolError(call.ID, "stage %q references unknown role %q", stage.Name, roleName)
			}

			taskID := roleTaskIDs[roleName]

			req := agent.BackgroundDispatchRequest{
				TaskID:         taskID,
				Description:    fmt.Sprintf("[%s/%s] %s", teamName, roleName, truncateGoal(goal, 80)),
				Prompt:         plan.prompt,
				AgentType:      canonicalAgentType(plan.role.AgentType),
				ExecutionMode:  normalizeExecutionMode(plan.role.ExecutionMode),
				AutonomyLevel:  normalizeAutonomy(plan.role.AutonomyLevel),
				CausationID:    call.ID,
				Config:         ports.CloneStringMap(plan.taskConfig),
				DependsOn:      stageDeps[roleName],
				WorkspaceMode:  agent.WorkspaceMode(plan.role.WorkspaceMode),
				InheritContext: plan.role.InheritContext,
			}

			if err := dispatcher.Dispatch(ctx, req); err != nil {
				msg := fmt.Sprintf("dispatch failed for role %q: %v", roleName, err)
				if len(dispatchedIDs) > 0 {
					msg += fmt.Sprintf(" (already dispatched: %v — use bg_status to monitor or cancel)", dispatchedIDs)
				}
				teamRunRecord.DispatchState = "partial_failure"
				teamRunRecord.Error = err.Error()
				recordPath, recordErr := persistTeamRunRecord(ctx, recorder, teamRunRecord)
				metadata := map[string]any{
					"partial_dispatch": dispatchedIDs,
					"team_run_id":      teamRunRecord.RunID,
				}
				if strings.TrimSpace(recordPath) != "" {
					metadata["team_run_record_path"] = recordPath
				}
				if recordErr != nil {
					metadata["team_run_record_error"] = recordErr.Error()
					msg += fmt.Sprintf(" (team run record failed: %v)", recordErr)
				}
				return &ports.ToolResult{
					CallID:   call.ID,
					Content:  msg,
					Error:    err,
					Metadata: metadata,
				}, nil
			}
			dispatchedIDs = append(dispatchedIDs, taskID)
		}
	}

	content := formatTeamSummary(team, roleTaskIDs, dispatchedIDs)
	recordPath, recordErr := persistTeamRunRecord(ctx, recorder, teamRunRecord)
	if recordErr != nil {
		content += fmt.Sprintf("\n\nWarning: failed to persist team run record: %v", recordErr)
	}
	metadata := map[string]any{
		"team":          teamName,
		"task_ids":      dispatchedIDs,
		"role_task_ids": roleTaskIDs,
		"team_run_id":   teamRunRecord.RunID,
	}
	if strings.TrimSpace(recordPath) != "" {
		metadata["team_run_record_path"] = recordPath
	}
	if recordErr != nil {
		metadata["team_run_record_error"] = recordErr.Error()
	}
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

func (t *teamDispatch) listTeams(callID string, teams []agent.TeamDefinition) (*ports.ToolResult, error) {
	if len(teams) == 0 {
		return &ports.ToolResult{
			CallID:  callID,
			Content: "No teams configured. Add teams under external_agents.teams in config.",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Available teams (%d):\n\n", len(teams)))
	for _, team := range teams {
		sb.WriteString(fmt.Sprintf("  %s — %s\n", team.Name, team.Description))
		sb.WriteString("    Roles: ")
		for i, role := range team.Roles {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s(%s)", role.Name, role.AgentType))
		}
		sb.WriteString("\n    Stages: ")
		for i, stage := range team.Stages {
			if i > 0 {
				sb.WriteString(" → ")
			}
			sb.WriteString(fmt.Sprintf("%s[%s]", stage.Name, strings.Join(stage.Roles, ",")))
		}
		sb.WriteString("\n\n")
	}
	return &ports.ToolResult{CallID: callID, Content: sb.String()}, nil
}

func findTeam(teams []agent.TeamDefinition, name string) *agent.TeamDefinition {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for i := range teams {
		if strings.ToLower(teams[i].Name) == normalized {
			return &teams[i]
		}
	}
	return nil
}

func findRole(roles []agent.TeamRoleDefinition, name string) *agent.TeamRoleDefinition {
	for i := range roles {
		if roles[i].Name == name {
			return &roles[i]
		}
	}
	return nil
}

func validateTeam(team *agent.TeamDefinition) error {
	if len(team.Roles) == 0 {
		return fmt.Errorf("team %q has no roles defined", team.Name)
	}
	if len(team.Stages) == 0 {
		return fmt.Errorf("team %q has no stages defined", team.Name)
	}

	roleSet := make(map[string]struct{}, len(team.Roles))
	for _, role := range team.Roles {
		if role.Name == "" {
			return fmt.Errorf("team %q has a role with empty name", team.Name)
		}
		if role.AgentType == "" {
			return fmt.Errorf("team %q role %q has no agent_type", team.Name, role.Name)
		}
		if _, exists := roleSet[role.Name]; exists {
			return fmt.Errorf("team %q has duplicate role name %q", team.Name, role.Name)
		}
		roleSet[role.Name] = struct{}{}
	}

	// Track which roles are referenced by stages.
	referencedRoles := make(map[string]int, len(team.Roles))
	for _, stage := range team.Stages {
		if len(stage.Roles) == 0 {
			return fmt.Errorf("team %q stage %q has no roles", team.Name, stage.Name)
		}
		for _, roleName := range stage.Roles {
			if _, ok := roleSet[roleName]; !ok {
				return fmt.Errorf("team %q stage %q references unknown role %q", team.Name, stage.Name, roleName)
			}
			referencedRoles[roleName]++
		}
	}

	// Each role must appear in exactly one stage.
	for _, role := range team.Roles {
		count := referencedRoles[role.Name]
		if count == 0 {
			return fmt.Errorf("team %q role %q is not assigned to any stage", team.Name, role.Name)
		}
		if count > 1 {
			return fmt.Errorf("team %q role %q appears in %d stages (must be exactly 1)", team.Name, role.Name, count)
		}
	}

	return nil
}

// buildStageDeps computes DependsOn for each role based on stage ordering.
// Roles in stage N+1 depend on all task IDs from stage N.
func buildStageDeps(stages []agent.TeamStageDefinition, roleTaskIDs map[string]string) map[string][]string {
	deps := make(map[string][]string)
	for i, stage := range stages {
		if i == 0 {
			// First stage has no dependencies.
			for _, roleName := range stage.Roles {
				deps[roleName] = nil
			}
			continue
		}
		// Collect all task IDs from the previous stage.
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
	// If caller provided a full prompt override for this role, use it.
	if override, ok := overrides[roleName]; ok && utils.HasContent(override) {
		return override
	}

	if utils.IsBlank(template) {
		return goal
	}

	result := template
	result = strings.ReplaceAll(result, "{GOAL}", goal)
	result = strings.ReplaceAll(result, "{ROLE}", roleName)
	result = strings.ReplaceAll(result, "{TEAM}", teamName)
	return result
}

func buildTeamTaskConfig(role *agent.TeamRoleDefinition, globalOverrides map[string]string) map[string]string {
	config := make(map[string]string)

	// Start with role-defined config.
	for k, v := range role.Config {
		config[k] = v
	}

	// Apply global overrides.
	for k, v := range globalOverrides {
		config[k] = v
	}

	// Ensure execution_mode and autonomy_level are always present.
	if _, ok := config["execution_mode"]; !ok {
		config["execution_mode"] = normalizeExecutionMode(role.ExecutionMode)
	}
	if _, ok := config["autonomy_level"]; !ok {
		config["autonomy_level"] = normalizeAutonomy(role.AutonomyLevel)
	}

	return config
}

func normalizeExecutionMode(raw string) string {
	return executioncontrol.NormalizeExecutionMode(raw)
}

func normalizeAutonomy(raw string) string {
	return executioncontrol.NormalizeAutonomyLevel(raw)
}

func truncateGoal(goal string, maxRunes int) string {
	runes := []rune(goal)
	if len(runes) <= maxRunes {
		return goal
	}
	return string(runes[:maxRunes-3]) + "..."
}

func formatTeamSummary(team *agent.TeamDefinition, roleTaskIDs map[string]string, dispatched []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Team %q dispatched (%d tasks).\n\n", team.Name, len(dispatched)))
	sb.WriteString("Workflow:\n")
	for i, stage := range team.Stages {
		if i > 0 {
			sb.WriteString("  ↓\n")
		}
		sb.WriteString(fmt.Sprintf("  Stage %d [%s]:\n", i+1, stage.Name))
		for _, roleName := range stage.Roles {
			taskID := roleTaskIDs[roleName]
			role := findRole(team.Roles, roleName)
			agentType := ""
			if role != nil {
				agentType = role.AgentType
			}
			sb.WriteString(fmt.Sprintf("    %s (%s) → %s\n", roleName, agentType, taskID))
		}
	}
	sb.WriteString(fmt.Sprintf("\nUse bg_status(task_ids=%v) to check progress.", dispatched))
	return sb.String()
}

func persistTeamRunRecord(
	ctx context.Context,
	recorder agent.TeamRunRecorder,
	record agent.TeamRunRecord,
) (string, error) {
	if recorder == nil {
		return "", nil
	}
	return recorder.RecordTeamRun(ctx, record)
}

func buildTeamRunStageRecords(stages []agent.TeamStageDefinition) []agent.TeamRunStageRecord {
	if len(stages) == 0 {
		return nil
	}
	out := make([]agent.TeamRunStageRecord, 0, len(stages))
	for _, stage := range stages {
		out = append(out, agent.TeamRunStageRecord{
			Name:  stage.Name,
			Roles: append([]string(nil), stage.Roles...),
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
