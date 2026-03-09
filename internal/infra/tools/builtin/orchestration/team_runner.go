package orchestration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/teamruntime"
	toolshared "alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
	"gopkg.in/yaml.v3"
)

const (
	fmtAllTasksCompleted = "全部 %d 个任务已完成（计划 %s）。\n"
	fmtTasksDispatched   = "已派发 %d 个后台任务（计划 %s）。\n"
	fmtStatusFileHint    = "\n任务在后台运行中，进度状态文件：%s"
)

type RunRequest struct {
	Dispatcher      agent.BackgroundTaskDispatcher
	FilePath        string
	TemplateName    string
	Goal            string
	PromptOverrides map[string]string
	Wait            bool
	Timeout         time.Duration
	Mode            taskfile.ExecutionMode
	TaskIDs         []string
	CausationID     string
	SessionID       string
	TeamDefinitions []agent.TeamDefinition
}

type RunResult struct {
	ExecuteResult *taskfile.ExecuteResult
	Content       string
}

type TeamRunner struct{}

func NewTeamRunner() *TeamRunner {
	return &TeamRunner{}
}

func (r *TeamRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if req.Dispatcher == nil {
		return nil, fmt.Errorf("background task dispatch is not available in this context")
	}

	req.FilePath = strings.TrimSpace(req.FilePath)
	req.TemplateName = strings.TrimSpace(req.TemplateName)
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Timeout <= 0 {
		req.Timeout = 120 * time.Second
	}
	switch req.Mode {
	case taskfile.ModeTeam, taskfile.ModeSwarm:
	case taskfile.ModeAuto, "":
		req.Mode = taskfile.ModeAuto
	default:
		return nil, fmt.Errorf("invalid mode %q: must be team, swarm, or auto", req.Mode)
	}

	if req.FilePath == "" && req.TemplateName == "" {
		return nil, fmt.Errorf("exactly one of file or template is required")
	}
	if req.FilePath != "" && req.TemplateName != "" {
		return nil, fmt.Errorf("file and template are mutually exclusive")
	}
	if strings.EqualFold(req.TemplateName, "list") {
		return &RunResult{Content: r.ListTemplates(req.TeamDefinitions)}, nil
	}
	if req.CausationID == "" {
		req.CausationID = "team-run-" + id.NewKSUID()
	}

	if req.TemplateName != "" {
		return r.executeFromTemplate(ctx, req)
	}
	return r.executeFromFile(ctx, req)
}

func (r *TeamRunner) ListTemplates(teams []agent.TeamDefinition) string {
	if len(teams) == 0 {
		return "No team templates configured."
	}

	var sb strings.Builder
	sb.WriteString("Available team templates:\n\n")
	for _, team := range teams {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", team.Name, team.Description))
		sb.WriteString("  Roles: ")
		for j, role := range team.Roles {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s (%s)", role.Name, role.AgentType))
		}
		sb.WriteString("\n  Stages: ")
		for j, stage := range team.Stages {
			if j > 0 {
				sb.WriteString(" → ")
			}
			sb.WriteString(strings.Join(stage.Roles, "+"))
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func (r *TeamRunner) executeFromFile(ctx context.Context, req RunRequest) (*RunResult, error) {
	tf, err := r.loadTaskFile(req.FilePath)
	if err != nil {
		return nil, err
	}
	if len(req.TaskIDs) > 0 {
		tf = taskfile.FilterTasks(tf, req.TaskIDs)
		if len(tf.Tasks) == 0 {
			return nil, fmt.Errorf("no matching task IDs found in file")
		}
	}

	statusPath := statusPathForFile(ctx, req.FilePath, tf.PlanID)
	result, err := r.executeTasks(ctx, req, tf, statusPath)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}
	return &RunResult{
		ExecuteResult: result,
		Content:       formatRunContent(result, req.Wait),
	}, nil
}

func (r *TeamRunner) executeFromTemplate(ctx context.Context, req RunRequest) (*RunResult, error) {
	teamDef, err := resolveTemplate(req.TeamDefinitions, req.TemplateName)
	if err != nil {
		return nil, err
	}
	if req.Goal == "" {
		return nil, fmt.Errorf("goal is required when using a template")
	}

	statusPath := statusPathForFile(ctx, "", fmt.Sprintf("team-%s", teamDef.Name))
	runResult, err := taskfile.DispatchTeamRun(ctx, taskfile.TeamRunRequest{
		Dispatcher:      req.Dispatcher,
		TeamDef:         teamDef,
		Goal:            req.Goal,
		PromptOverrides: req.PromptOverrides,
		CausationID:     req.CausationID,
		StatusPath:      statusPath,
		Mode:            req.Mode,
		Wait:            req.Wait,
		Timeout:         req.Timeout,
		TaskIDs:         req.TaskIDs,
		BootstrapFn: func(ctx context.Context, tf *taskfile.TaskFile) error {
			bootstrap, err := r.ensureTeamBootstrap(ctx, statusPath, req.TemplateName, req.Goal, *teamDef, req.SessionID)
			if err != nil {
				return err
			}
			applyBootstrapToTaskFile(tf, bootstrap)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	if recorder := agent.GetTeamRunRecorder(ctx); recorder != nil {
		if _, recErr := recorder.RecordTeamRun(ctx, runResult.Record); recErr != nil {
			_ = recErr
		}
	}

	return &RunResult{
		ExecuteResult: runResult.ExecuteResult,
		Content:       formatRunContent(runResult.ExecuteResult, req.Wait),
	}, nil
}

func (r *TeamRunner) executeTasks(ctx context.Context, req RunRequest, tf *taskfile.TaskFile, statusPath string) (*taskfile.ExecuteResult, error) {
	executor := taskfile.NewExecutor(req.Dispatcher, req.Mode, taskfile.DefaultSwarmConfig())
	if req.Wait {
		return executor.ExecuteAndWait(ctx, tf, req.CausationID, statusPath, req.Timeout)
	}
	return executor.Execute(ctx, tf, req.CausationID, statusPath)
}

func (r *TeamRunner) loadTaskFile(path string) (*taskfile.TaskFile, error) {
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

func (r *TeamRunner) ensureTeamBootstrap(
	ctx context.Context,
	statusPath string,
	templateName string,
	goal string,
	teamDef agent.TeamDefinition,
	sessionID string,
) (*teamruntime.EnsureResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID, _ = toolshared.GetSessionID(ctx)
	}
	if sessionID == "" {
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

func resolveTemplate(teams []agent.TeamDefinition, templateName string) (*agent.TeamDefinition, error) {
	for i := range teams {
		if strings.EqualFold(teams[i].Name, templateName) {
			return &teams[i], nil
		}
	}
	return nil, fmt.Errorf("template %q not found. Use `alex team run --template list` to see available templates", templateName)
}

func formatRunContent(result *taskfile.ExecuteResult, waited bool) string {
	var sb strings.Builder
	if waited {
		sb.WriteString(fmt.Sprintf(fmtAllTasksCompleted, len(result.TaskIDs), result.PlanID))
	} else {
		sb.WriteString(fmt.Sprintf(fmtTasksDispatched, len(result.TaskIDs), result.PlanID))
	}
	for _, taskID := range result.TaskIDs {
		sb.WriteString(fmt.Sprintf("- %s\n", taskID))
	}
	if !waited {
		sb.WriteString(fmt.Sprintf(fmtStatusFileHint, result.StatusPath))
	}
	return sb.String()
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
	return filepath.Join(".elephant", "tasks", planID+".status.yaml")
}
