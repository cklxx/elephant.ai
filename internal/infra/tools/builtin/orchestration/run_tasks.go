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

// execParams bundles shared execution parameters parsed from the tool call.
type execParams struct {
	dispatcher agent.BackgroundTaskDispatcher
	wait       bool
	timeout    time.Duration
	mode       taskfile.ExecutionMode
	taskIDs    []string
}

func parseExecParams(call ports.ToolCall, dispatcher agent.BackgroundTaskDispatcher) (execParams, error) {
	ep := execParams{
		dispatcher: dispatcher,
		timeout:    120 * time.Second,
		mode:       taskfile.ModeAuto,
	}
	if raw, ok := call.Arguments["wait"]; ok {
		if v, ok := raw.(bool); ok {
			ep.wait = v
		}
	}
	if raw, ok := call.Arguments["timeout_seconds"]; ok {
		if v, err := parseOptionalInt(raw, "timeout_seconds"); err == nil && v > 0 {
			ep.timeout = time.Duration(v) * time.Second
		}
	}
	if raw, ok := call.Arguments["mode"].(string); ok {
		switch taskfile.ExecutionMode(strings.TrimSpace(raw)) {
		case taskfile.ModeTeam:
			ep.mode = taskfile.ModeTeam
		case taskfile.ModeSwarm:
			ep.mode = taskfile.ModeSwarm
		case taskfile.ModeAuto, "":
			// already default
		default:
			return ep, fmt.Errorf("invalid mode %q: must be team, swarm, or auto", raw)
		}
	}
	if raw, ok := call.Arguments["task_ids"]; ok {
		ids, err := parseStringList(raw, "task_ids")
		if err != nil {
			return ep, fmt.Errorf("task_ids: %s", err)
		}
		ep.taskIDs = ids
	}
	return ep, nil
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
	if strings.EqualFold(templateName, "list") {
		return t.listTemplates(ctx, call.ID)
	}

	ep, err := parseExecParams(call, dispatcher)
	if err != nil {
		return shared.ToolError(call.ID, "%s", err)
	}

	if templateName != "" {
		return t.executeFromTemplate(ctx, call, templateName, ep)
	}
	return t.executeFromFile(ctx, call.ID, filePath, ep)
}

// executeFromFile loads a YAML task file and executes its tasks.
func (t *runTasks) executeFromFile(ctx context.Context, callID string, filePath string, ep execParams) (*ports.ToolResult, error) {
	tf, err := t.loadTaskFile(filePath)
	if err != nil {
		return shared.ToolError(callID, "%s", err)
	}

	if len(ep.taskIDs) > 0 {
		tf = taskfile.FilterTasks(tf, ep.taskIDs)
		if len(tf.Tasks) == 0 {
			return shared.ToolError(callID, "no matching task IDs found in file")
		}
	}

	statusPath := statusPathForFile(ctx, filePath, tf.PlanID)
	result, err := t.executeTasks(ctx, callID, tf, statusPath, ep)
	if err != nil {
		return shared.ToolError(callID, "execution failed: %s", err)
	}
	return t.formatResult(callID, result, ep.wait)
}

// executeFromTemplate resolves a team template, bootstraps the runtime, and executes.
func (t *runTasks) executeFromTemplate(ctx context.Context, call ports.ToolCall, templateName string, ep execParams) (*ports.ToolResult, error) {
	goal, teamDef, overrides, err := t.resolveTemplateArgs(ctx, call, templateName)
	if err != nil {
		return shared.ToolError(call.ID, "%s", err)
	}

	statusPath := statusPathForFile(ctx, "", fmt.Sprintf("team-%s", teamDef.Name))

	runResult, err := taskfile.DispatchTeamRun(ctx, taskfile.TeamRunRequest{
		Dispatcher:      ep.dispatcher,
		TeamDef:         teamDef,
		Goal:            goal,
		PromptOverrides: overrides,
		CausationID:     call.ID,
		StatusPath:      statusPath,
		Mode:            ep.mode,
		Wait:            ep.wait,
		Timeout:         ep.timeout,
		TaskIDs:         ep.taskIDs,
		BootstrapFn: func(ctx context.Context, tf *taskfile.TaskFile) error {
			bootstrap, err := t.ensureTeamBootstrap(ctx, statusPath, templateName, goal, *teamDef)
			if err != nil {
				return err
			}
			applyBootstrapToTaskFile(tf, bootstrap)
			return nil
		},
	})
	if err != nil {
		return shared.ToolError(call.ID, "execution failed: %s", err)
	}

	if recorder := agent.GetTeamRunRecorder(ctx); recorder != nil {
		if _, recErr := recorder.RecordTeamRun(ctx, runResult.Record); recErr != nil {
			_ = recErr // best-effort
		}
	}

	return t.formatResult(call.ID, runResult.ExecuteResult, ep.wait)
}

// executeTasks creates an executor and runs the task file.
func (t *runTasks) executeTasks(ctx context.Context, callID string, tf *taskfile.TaskFile, statusPath string, ep execParams) (*taskfile.ExecuteResult, error) {
	executor := taskfile.NewExecutor(ep.dispatcher, ep.mode, taskfile.DefaultSwarmConfig())
	if ep.wait {
		return executor.ExecuteAndWait(ctx, tf, callID, statusPath, ep.timeout)
	}
	return executor.Execute(ctx, tf, callID, statusPath)
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

func (t *runTasks) resolveTemplateArgs(ctx context.Context, call ports.ToolCall, templateName string) (string, *agent.TeamDefinition, map[string]string, error) {
	goal, _ := call.Arguments["goal"].(string)
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return "", nil, nil, fmt.Errorf("goal is required when using a template")
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
		return "", nil, nil, fmt.Errorf("template %q not found. Use template=\"list\" to see available templates", templateName)
	}

	var overrides map[string]string
	if raw, ok := call.Arguments["prompts"]; ok {
		parsed, err := parseStringMap(raw, "prompts")
		if err != nil {
			return "", nil, nil, fmt.Errorf("prompts: %w", err)
		}
		overrides = parsed
	}

	return goal, def, overrides, nil
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
		if !shouldBootstrapRole(role) {
			continue
		}
		roleIDs = append(roleIDs, roleID)
		profiles[roleID] = strings.TrimSpace(role.CapabilityProfile)
		targets[roleID] = bootstrapTargetCLI(role)
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

func bootstrapTargetCLI(role agent.TeamRoleDefinition) string {
	if target := strings.TrimSpace(role.TargetCLI); target != "" {
		return target
	}
	// Respect explicit external agent_type as the bootstrap target so role
	// routing remains deterministic and doesn't drift to another CLI.
	if agent.IsCodingExternalAgent(role.AgentType) {
		return strings.TrimSpace(role.AgentType)
	}
	return ""
}

func shouldBootstrapRole(role agent.TeamRoleDefinition) bool {
	if strings.TrimSpace(role.TargetCLI) != "" {
		return true
	}
	return agent.IsCodingExternalAgent(role.AgentType)
}

func applyBootstrapToTaskFile(tf *taskfile.TaskFile, bootstrap *teamruntime.EnsureResult) {
	if tf == nil || bootstrap == nil {
		return
	}
	for i := range tf.Tasks {
		roleID := taskfile.ExtractRoleID(tf.Tasks[i].ID)
		if roleID == "" {
			continue
		}
		binding, ok := bootstrap.RoleBindings[roleID]
		if !ok {
			continue
		}
		tf.Tasks[i].RuntimeMeta = taskfile.TeamRuntimeMeta{
			TeamID:            bootstrap.Bootstrap.TeamID,
			RoleID:            roleID,
			TeamRuntimeDir:    bootstrap.BaseDir,
			TeamEventLog:      bootstrap.EventLogPath,
			CapabilityProfile: binding.CapabilityProfile,
			TargetCLI:         binding.TargetCLI,
			SelectedCLI:       binding.SelectedCLI,
			FallbackCLIs:      binding.FallbackCLIs,
			Binary:            binding.SelectedPath,
			RoleLogPath:       binding.RoleLogPath,
			TmuxSession:       bootstrap.Bootstrap.TmuxSession,
			TmuxPane:          binding.TmuxPane,
			SelectedAgentType: binding.SelectedAgentType,
		}
	}
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
