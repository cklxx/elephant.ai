package orchestration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/teamruntime"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
	"gopkg.in/yaml.v3"
)

// Chinese format strings for user-facing output.
const (
	fmtAllTasksCompleted = "全部 %d 个任务已完成（计划 %s）。\n"
	fmtTasksDispatched   = "已派发 %d 个后台任务（计划 %s）。\n"
	fmtStatusFileHint    = "\n任务在后台运行中，进度状态文件：%s"
)

type runTasks struct {
	shared.BaseTool
}

// NewRunTasks creates the run_tasks tool for file-based orchestration.
func NewRunTasks() *runTasks {
	return &runTasks{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "run_tasks",
				Description: `Execute tasks defined in a YAML task file. The agent writes a task file using write_file, then calls run_tasks to dispatch all tasks to background agents. Status is written to a .status sidecar file readable via read_file.

Supports two input modes:
- file: Read a TaskFile YAML and dispatch its tasks
- template: Use a pre-configured team template with a goal

Execution strategy (mode parameter):
- team: Sequential dispatch with dependency blocking and context inheritance. Best for tightly-coupled collaborative work.
- swarm: Stage-batched parallel execution with adaptive concurrency. Best for independent, embarrassingly-parallel tasks.
- auto (default): Analyzes the task DAG to pick team or swarm automatically.

Use wait=true for synchronous execution (blocks until all tasks complete).`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"file": {
							Type:        "string",
							Description: "Path to TaskFile YAML. Mutually exclusive with template.",
						},
						"template": {
							Type:        "string",
							Description: `Team template name. Mutually exclusive with file. Pass "list" to see available templates.`,
						},
						"goal": {
							Type:        "string",
							Description: "Goal for the team template. Required when template is provided.",
						},
						"prompts": {
							Type:        "object",
							Description: "Per-role prompt overrides when using a template (role_name -> prompt).",
						},
						"wait": {
							Type:        "boolean",
							Description: "Block until all tasks complete. Default: false.",
						},
						"timeout_seconds": {
							Type:        "integer",
							Description: "Max wait time in seconds when wait=true. Default: 120.",
						},
						"task_ids": {
							Type:        "array",
							Description: "Only execute specific task IDs from the file. Omit to execute all.",
							Items:       &ports.Property{Type: "string"},
						},
						"mode": {
							Type:        "string",
							Description: `Execution strategy: "team" (sequential with deps), "swarm" (parallel batches with adaptive concurrency), "auto" (analyze DAG to select). Default: "auto".`,
							Enum:        []any{"team", "swarm", "auto"},
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "run_tasks",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"orchestration", "background"},
			},
		),
	}
}

func (t *runTasks) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}

	filePath, _ := call.Arguments["file"].(string)
	templateName, _ := call.Arguments["template"].(string)
	filePath = strings.TrimSpace(filePath)
	templateName = strings.TrimSpace(templateName)

	if filePath == "" && templateName == "" {
		return shared.ToolError(call.ID, "exactly one of file or template is required")
	}
	if filePath != "" && templateName != "" {
		return shared.ToolError(call.ID, "file and template are mutually exclusive")
	}

	// Handle template listing.
	if strings.EqualFold(templateName, "list") {
		return t.listTemplates(ctx, call.ID)
	}

	var tf *taskfile.TaskFile
	var teamDef *agent.TeamDefinition
	var err error

	if templateName != "" {
		tf, teamDef, err = t.resolveTemplate(ctx, call, templateName)
	} else {
		tf, err = t.loadTaskFile(filePath)
	}
	if err != nil {
		return shared.ToolError(call.ID, "%s", err)
	}

	// Filter to specific task IDs if requested.
	if raw, ok := call.Arguments["task_ids"]; ok {
		ids, parseErr := parseStringList(raw, "task_ids")
		if parseErr != nil {
			return shared.ToolError(call.ID, "task_ids: %s", parseErr)
		}
		if len(ids) > 0 {
			tf = filterTasks(tf, ids)
			if len(tf.Tasks) == 0 {
				return shared.ToolError(call.ID, "no matching task IDs found in file")
			}
		}
	}

	wait := false
	if raw, ok := call.Arguments["wait"]; ok {
		if v, ok := raw.(bool); ok {
			wait = v
		}
	}
	timeout := 120 * time.Second
	if raw, ok := call.Arguments["timeout_seconds"]; ok {
		if v, err := parseOptionalInt(raw, "timeout_seconds"); err == nil && v > 0 {
			timeout = time.Duration(v) * time.Second
		}
	}

	// Parse execution mode.
	mode := taskfile.ModeAuto
	if raw, ok := call.Arguments["mode"].(string); ok {
		switch taskfile.ExecutionMode(strings.TrimSpace(raw)) {
		case taskfile.ModeTeam:
			mode = taskfile.ModeTeam
		case taskfile.ModeSwarm:
			mode = taskfile.ModeSwarm
		case taskfile.ModeAuto, "":
			mode = taskfile.ModeAuto
		default:
			return shared.ToolError(call.ID, "invalid mode %q: must be team, swarm, or auto", raw)
		}
	}

	// Determine status path. Kernel contexts override the default .elephant/tasks/ base dir.
	statusPath := statusPathForFile(ctx, filePath, tf.PlanID)

	if templateName != "" && teamDef != nil {
		goal, _ := call.Arguments["goal"].(string)
		bootstrap, bootstrapErr := t.ensureTeamBootstrap(ctx, statusPath, templateName, strings.TrimSpace(goal), *teamDef)
		if bootstrapErr != nil {
			return shared.ToolError(call.ID, "team bootstrap failed: %s", bootstrapErr)
		}
		applyBootstrapToTaskFile(tf, bootstrap)
	}

	executor := taskfile.NewExecutor(dispatcher, mode, taskfile.DefaultSwarmConfig())
	var result *taskfile.ExecuteResult

	if wait {
		result, err = executor.ExecuteAndWait(ctx, tf, call.ID, statusPath, timeout)
	} else {
		result, err = executor.Execute(ctx, tf, call.ID, statusPath)
	}
	if err != nil {
		return shared.ToolError(call.ID, "execution failed: %s", err)
	}

	if templateName != "" {
		if recorder := agent.GetTeamRunRecorder(ctx); recorder != nil {
			goal, _ := call.Arguments["goal"].(string)
			record := buildTeamRunRecord(tf, teamDef, templateName, strings.TrimSpace(goal), result, statusPath, wait)
			if _, recErr := recorder.RecordTeamRun(ctx, record); recErr != nil {
				_ = recErr // best-effort
			}
		}
	}

	return t.formatResult(call.ID, result, wait)
}

func (t *runTasks) loadTaskFile(path string) (*taskfile.TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task file: %w", err)
	}
	var tf taskfile.TaskFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parse task file: %w", err)
	}
	return &tf, nil
}

func (t *runTasks) resolveTemplate(ctx context.Context, call ports.ToolCall, templateName string) (*taskfile.TaskFile, *agent.TeamDefinition, error) {
	goal, _ := call.Arguments["goal"].(string)
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return nil, nil, fmt.Errorf("goal is required when using a template")
	}

	teams := agent.GetTeamDefinitions(ctx)
	var def *agent.TeamDefinition
	for i := range teams {
		if strings.EqualFold(teams[i].Name, templateName) {
			def = &teams[i]
			break
		}
	}
	if def == nil {
		return nil, nil, fmt.Errorf("template %q not found. Use template=\"list\" to see available templates", templateName)
	}

	tmpl := taskfile.TeamTemplateFromDefinition(*def)

	var overrides map[string]string
	if raw, ok := call.Arguments["prompts"]; ok {
		parsed, err := parseStringMap(raw, "prompts")
		if err != nil {
			return nil, nil, fmt.Errorf("prompts: %w", err)
		}
		overrides = parsed
	}

	return taskfile.RenderTaskFile(&tmpl, goal, overrides), def, nil
}

func (t *runTasks) listTemplates(ctx context.Context, callID string) (*ports.ToolResult, error) {
	teams := agent.GetTeamDefinitions(ctx)
	if len(teams) == 0 {
		return &ports.ToolResult{
			CallID:  callID,
			Content: "No team templates configured.",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString("Available team templates:\n\n")
	for _, team := range teams {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", team.Name, team.Description))
		sb.WriteString("  Roles: ")
		for j, r := range team.Roles {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s (%s)", r.Name, r.AgentType))
		}
		sb.WriteString("\n  Stages: ")
		for j, s := range team.Stages {
			if j > 0 {
				sb.WriteString(" → ")
			}
			sb.WriteString(strings.Join(s.Roles, "+"))
		}
		sb.WriteString("\n\n")
	}
	return &ports.ToolResult{CallID: callID, Content: sb.String()}, nil
}

func (t *runTasks) formatResult(callID string, result *taskfile.ExecuteResult, waited bool) (*ports.ToolResult, error) {
	var sb strings.Builder
	if waited {
		sb.WriteString(fmt.Sprintf(fmtAllTasksCompleted, len(result.TaskIDs), result.PlanID))
	} else {
		sb.WriteString(fmt.Sprintf(fmtTasksDispatched, len(result.TaskIDs), result.PlanID))
	}
	for _, id := range result.TaskIDs {
		sb.WriteString(fmt.Sprintf("- %s\n", id))
	}
	if !waited {
		sb.WriteString(fmt.Sprintf(fmtStatusFileHint, result.StatusPath))
	}
	return &ports.ToolResult{CallID: callID, Content: sb.String()}, nil
}

func filterTasks(tf *taskfile.TaskFile, ids []string) *taskfile.TaskFile {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	filtered := &taskfile.TaskFile{
		Version:  tf.Version,
		PlanID:   tf.PlanID,
		Defaults: tf.Defaults,
		Metadata: tf.Metadata,
	}
	for _, t := range tf.Tasks {
		if idSet[t.ID] {
			// Remove DependsOn references to tasks not in the filtered set.
			var cleanDeps []string
			for _, dep := range t.DependsOn {
				if idSet[dep] {
					cleanDeps = append(cleanDeps, dep)
				}
			}
			t.DependsOn = cleanDeps
			filtered.Tasks = append(filtered.Tasks, t)
		}
	}
	return filtered
}

func buildTeamRunRecord(tf *taskfile.TaskFile, def *agent.TeamDefinition, templateName, goal string, result *taskfile.ExecuteResult, statusPath string, waited bool) agent.TeamRunRecord {
	state := "dispatched"
	if waited {
		state = dispatchStateFromStatus(statusPath)
	}
	var stages []agent.TeamRunStageRecord
	var roles []agent.TeamRunRoleRecord

	// Populate stages from the resolved team definition.
	if def != nil {
		for _, s := range def.Stages {
			stages = append(stages, agent.TeamRunStageRecord{
				Name:  s.Name,
				Roles: s.Roles,
			})
		}
	}

	// Populate roles from the rendered TaskFile tasks.
	for _, t := range tf.Tasks {
		roles = append(roles, agent.TeamRunRoleRecord{
			Name:              t.ID,
			AgentType:         t.AgentType,
			CapabilityProfile: strings.TrimSpace(t.Config["capability_profile"]),
			TargetCLI:         strings.TrimSpace(t.Config["target_cli"]),
			SelectedCLI:       strings.TrimSpace(t.Config["selected_cli"]),
			FallbackCLIs:      splitCSV(t.Config["fallback_clis"]),
			TaskID:            t.ID,
			DependsOn:         t.DependsOn,
			ExecutionMode:     t.ExecutionMode,
			AutonomyLevel:     t.AutonomyLevel,
			WorkspaceMode:     t.WorkspaceMode,
			InheritContext:    t.InheritContext,
			Config:            t.Config,
		})
	}
	return agent.TeamRunRecord{
		TeamName:      templateName,
		Goal:          goal,
		CausationID:   result.PlanID,
		DispatchedAt:  time.Now(),
		DispatchState: state,
		Stages:        stages,
		Roles:         roles,
	}
}

func (t *runTasks) ensureTeamBootstrap(
	ctx context.Context,
	statusPath string,
	templateName string,
	goal string,
	teamDef agent.TeamDefinition,
) (*teamruntime.EnsureResult, error) {
	sessionID, _ := shared.GetSessionID(ctx)
	if strings.TrimSpace(sessionID) == "" {
		sessionID = id.SessionIDFromContext(ctx)
	}
	roleIDs := make([]string, 0, len(teamDef.Roles))
	profiles := make(map[string]string, len(teamDef.Roles))
	targets := make(map[string]string, len(teamDef.Roles))
	for _, role := range teamDef.Roles {
		roleID := strings.TrimSpace(role.Name)
		if roleID == "" {
			continue
		}
		roleIDs = append(roleIDs, roleID)
		profiles[roleID] = strings.TrimSpace(role.CapabilityProfile)
		targets[roleID] = strings.TrimSpace(role.TargetCLI)
	}

	baseDir := filepath.Join(filepath.Dir(statusPath), "_team_runtime")
	manager := teamruntime.NewBootstrapManager(baseDir, nil)
	return manager.Ensure(ctx, teamruntime.EnsureRequest{
		SessionID: sessionID,
		Template:  templateName,
		Goal:      goal,
		RoleIDs:   roleIDs,
		Profiles:  profiles,
		Targets:   targets,
	})
}

func applyBootstrapToTaskFile(tf *taskfile.TaskFile, bootstrap *teamruntime.EnsureResult) {
	if tf == nil || bootstrap == nil {
		return
	}
	for i := range tf.Tasks {
		roleID := extractRoleIDFromTaskID(tf.Tasks[i].ID)
		if roleID == "" {
			continue
		}
		binding, ok := bootstrap.RoleBindings[roleID]
		if !ok {
			continue
		}
		if tf.Tasks[i].Config == nil {
			tf.Tasks[i].Config = make(map[string]string)
		}
		tf.Tasks[i].Config["team_id"] = bootstrap.Bootstrap.TeamID
		tf.Tasks[i].Config["role_id"] = roleID
		tf.Tasks[i].Config["team_runtime_dir"] = bootstrap.BaseDir
		tf.Tasks[i].Config["team_event_log"] = bootstrap.EventLogPath
		if strings.TrimSpace(binding.CapabilityProfile) != "" {
			tf.Tasks[i].Config["capability_profile"] = strings.TrimSpace(binding.CapabilityProfile)
		}
		if strings.TrimSpace(binding.TargetCLI) != "" {
			tf.Tasks[i].Config["target_cli"] = strings.TrimSpace(binding.TargetCLI)
		}
		if strings.TrimSpace(binding.SelectedCLI) != "" {
			tf.Tasks[i].Config["selected_cli"] = strings.TrimSpace(binding.SelectedCLI)
		}
		if len(binding.FallbackCLIs) > 0 {
			tf.Tasks[i].Config["fallback_clis"] = strings.Join(binding.FallbackCLIs, ",")
		}
		if strings.TrimSpace(binding.SelectedPath) != "" {
			tf.Tasks[i].Config["binary"] = strings.TrimSpace(binding.SelectedPath)
		}
		if strings.TrimSpace(binding.RoleLogPath) != "" {
			tf.Tasks[i].Config["role_log_path"] = strings.TrimSpace(binding.RoleLogPath)
		}
		if strings.TrimSpace(bootstrap.Bootstrap.TmuxSession) != "" {
			tf.Tasks[i].Config["tmux_session"] = strings.TrimSpace(bootstrap.Bootstrap.TmuxSession)
		}
		if strings.TrimSpace(binding.TmuxPane) != "" {
			tf.Tasks[i].Config["tmux_pane"] = strings.TrimSpace(binding.TmuxPane)
		}
		if strings.TrimSpace(binding.SelectedAgentType) != "" {
			tf.Tasks[i].AgentType = strings.TrimSpace(binding.SelectedAgentType)
		}
	}
}

func extractRoleIDFromTaskID(taskID string) string {
	id := strings.TrimSpace(taskID)
	if !strings.HasPrefix(id, "team-") {
		return ""
	}
	trimmed := strings.TrimPrefix(id, "team-")
	trimmed = strings.TrimSuffix(trimmed, "-debate")
	for {
		idx := strings.LastIndex(trimmed, "-retry-")
		if idx <= 0 {
			break
		}
		suffix := strings.TrimSpace(trimmed[idx+len("-retry-"):])
		if suffix == "" || strings.IndexFunc(suffix, func(r rune) bool { return r < '0' || r > '9' }) != -1 {
			break
		}
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	return strings.TrimSpace(trimmed)
}

func splitCSV(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func dispatchStateFromStatus(statusPath string) string {
	sf, err := taskfile.ReadStatusFile(statusPath)
	if err != nil {
		return "unknown"
	}
	failed, completed := 0, 0
	for _, ts := range sf.Tasks {
		switch ts.Status {
		case "failed":
			failed++
		case "completed":
			completed++
		}
	}
	if failed > 0 {
		if completed > 0 {
			return "partial_failure"
		}
		return "failed"
	}
	return "completed"
}

func statusPathForFile(ctx context.Context, filePath, planID string) string {
	if filePath != "" {
		ext := filepath.Ext(filePath)
		return strings.TrimSuffix(filePath, ext) + ".status" + ext
	}
	if tasksDir := shared.KernelTasksDirFromContext(ctx); tasksDir != "" {
		return filepath.Join(tasksDir, planID+".status.yaml")
	}
	return filepath.Join(".elephant", "tasks", planID+".status.yaml")
}
