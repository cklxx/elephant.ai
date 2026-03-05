package taskfile

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// BootstrapFn is called after RenderTaskFile to inject runtime bindings
// (tmux panes, CLI selection, etc.) into the TaskFile before execution.
type BootstrapFn func(ctx context.Context, tf *TaskFile) error

// TeamRunRequest contains all inputs for dispatching a structured team run.
type TeamRunRequest struct {
	Dispatcher      agent.BackgroundTaskDispatcher
	TeamDef         *agent.TeamDefinition
	Goal            string
	PromptOverrides map[string]string
	CausationID     string
	StatusPath      string
	Mode            ExecutionMode
	Wait            bool
	Timeout         time.Duration
	SwarmConfig     SwarmConfig
	BootstrapFn     BootstrapFn
	TaskIDs         []string // optional: filter to specific task IDs after render
}

// TeamRunResult captures the outcome of a team dispatch.
type TeamRunResult struct {
	*ExecuteResult
	Record agent.TeamRunRecord
}

// DispatchTeamRun is the shared entry point for team execution.
// It renders a TaskFile from the team definition, applies optional bootstrap,
// executes via the Executor, and builds an audit record.
func DispatchTeamRun(ctx context.Context, req TeamRunRequest) (*TeamRunResult, error) {
	if req.TeamDef == nil {
		return nil, fmt.Errorf("team definition is required")
	}
	if strings.TrimSpace(req.Goal) == "" {
		return nil, fmt.Errorf("goal is required")
	}
	if req.Dispatcher == nil {
		return nil, fmt.Errorf("dispatcher is required")
	}

	tf := RenderTaskFile(req.TeamDef, req.Goal, req.PromptOverrides)

	if len(req.TaskIDs) > 0 {
		tf = FilterTasks(tf, req.TaskIDs)
		if len(tf.Tasks) == 0 {
			return nil, fmt.Errorf("no matching task IDs found")
		}
	}

	if req.BootstrapFn != nil {
		if err := req.BootstrapFn(ctx, tf); err != nil {
			return nil, fmt.Errorf("bootstrap: %w", err)
		}
	}

	swarmCfg := req.SwarmConfig
	if swarmCfg == (SwarmConfig{}) {
		swarmCfg = DefaultSwarmConfig()
	}

	executor := NewExecutor(req.Dispatcher, req.Mode, swarmCfg)
	var result *ExecuteResult
	var err error

	if req.Wait {
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = 120 * time.Second
		}
		result, err = executor.ExecuteAndWait(ctx, tf, req.CausationID, req.StatusPath, timeout)
	} else {
		result, err = executor.Execute(ctx, tf, req.CausationID, req.StatusPath)
	}
	if err != nil {
		return nil, err
	}

	record := BuildTeamRunRecord(tf, req.TeamDef, req.TeamDef.Name, req.Goal, result, req.StatusPath, req.Wait)

	return &TeamRunResult{
		ExecuteResult: result,
		Record:        record,
	}, nil
}

// BuildTeamRunRecord constructs an audit record from a completed team dispatch.
func BuildTeamRunRecord(tf *TaskFile, def *agent.TeamDefinition, templateName, goal string, result *ExecuteResult, statusPath string, waited bool) agent.TeamRunRecord {
	state := "dispatched"
	if waited {
		state = DispatchStateFromStatus(statusPath)
	}
	var stages []agent.TeamRunStageRecord
	var roles []agent.TeamRunRoleRecord

	if def != nil {
		for _, s := range def.Stages {
			stages = append(stages, agent.TeamRunStageRecord{
				Name:  s.Name,
				Roles: s.Roles,
			})
		}
	}

	for _, t := range tf.Tasks {
		agentType := t.AgentType
		if sel := strings.TrimSpace(t.RuntimeMeta.SelectedAgentType); sel != "" && agent.IsCodingExternalAgent(agentType) {
			agentType = sel
		}
		roles = append(roles, agent.TeamRunRoleRecord{
			Name:              t.ID,
			AgentType:         agentType,
			CapabilityProfile: strings.TrimSpace(t.RuntimeMeta.CapabilityProfile),
			TargetCLI:         strings.TrimSpace(t.RuntimeMeta.TargetCLI),
			SelectedCLI:       strings.TrimSpace(t.RuntimeMeta.SelectedCLI),
			FallbackCLIs:      t.RuntimeMeta.FallbackCLIs,
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

// FilterTasks returns a copy of the TaskFile containing only the specified task IDs.
// DependsOn references to excluded tasks are removed.
func FilterTasks(tf *TaskFile, ids []string) *TaskFile {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	filtered := &TaskFile{
		Version:  tf.Version,
		PlanID:   tf.PlanID,
		Defaults: tf.Defaults,
		Metadata: tf.Metadata,
	}
	for _, t := range tf.Tasks {
		if idSet[t.ID] {
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

// DispatchStateFromStatus reads a task status sidecar and returns a summary state string.
func DispatchStateFromStatus(statusPath string) string {
	sf, err := ReadStatusFile(statusPath)
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
