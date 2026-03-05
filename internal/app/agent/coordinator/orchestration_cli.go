package coordinator

import (
	"context"
	"fmt"
	"strings"

	appcontext "alex/internal/app/agent/context"
	agent "alex/internal/domain/agent/ports/agent"
	react "alex/internal/domain/agent/react"
	infraadapters "alex/internal/infra/adapters"
	infraruntime "alex/internal/infra/runtime"
	id "alex/internal/shared/utils/id"
)

// EnsureBackgroundDispatcher returns a background task dispatcher bound to the
// provided session. When the session has no manager yet, a new one is created
// using the same runtime wiring as ExecuteTask.
func (c *AgentCoordinator) EnsureBackgroundDispatcher(ctx context.Context, sessionID string) (agent.BackgroundTaskDispatcher, error) {
	if c == nil {
		return nil, fmt.Errorf("agent coordinator is nil")
	}
	if c.bgRegistry == nil {
		return nil, fmt.Errorf("background task registry not available")
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = id.SessionIDFromContext(ctx)
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}

	ctx = id.WithSessionID(ctx, sessionID)
	ctx, _ = id.EnsureRunID(ctx, id.NewRunID)

	logger := c.loggerFor(ctx)
	effectiveCfg := c.effectiveConfig(ctx)
	idAdapter := infraruntime.IDsAdapter{}
	goRunner := infraruntime.GoRunner
	workingDirResolver := infraruntime.WorkingDirResolver
	workspaceMgrFactory := infraruntime.WorkspaceManagerFactory

	backgroundExecutor := func(bgCtx context.Context, prompt, sid string, listener agent.EventListener) (*agent.TaskResult, error) {
		bgCtx = appcontext.MarkSubagentContext(bgCtx)
		return c.ExecuteTask(bgCtx, prompt, sid, listener)
	}

	mgr := c.bgRegistry.Get(sessionID, func() *react.BackgroundTaskManager {
		return react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
			RunContext:          ctx,
			Logger:              logger,
			Clock:               c.clock,
			IDGenerator:         idAdapter,
			IDContextReader:     idAdapter,
			GoRunner:            goRunner,
			WorkingDirResolver:  workingDirResolver,
			WorkspaceMgrFactory: workspaceMgrFactory,
			ExecuteTask:         backgroundExecutor,
			ExternalExecutor:    c.externalExecutor,
			SessionID:           sessionID,
			MaxConcurrentTasks:  effectiveCfg.MaxBackgroundTasks,
			ContextPropagators: []agent.ContextPropagatorFunc{
				appcontext.PropagateLLMSelection,
			},
			TmuxSender:    infraadapters.NewExecTmuxSender(),
			EventAppender: infraadapters.NewFileEventAppender(),
		})
	})
	if mgr == nil {
		return nil, fmt.Errorf("failed to initialize background dispatcher")
	}
	return mgr, nil
}

// TeamDefinitionsSnapshot returns a detached copy of configured team definitions
// for orchestration dispatch.
func (c *AgentCoordinator) TeamDefinitionsSnapshot() []agent.TeamDefinition {
	if c == nil || len(c.teamDefinitions) == 0 {
		return nil
	}
	out := make([]agent.TeamDefinition, len(c.teamDefinitions))
	for i, team := range c.teamDefinitions {
		clone := team
		if len(team.Roles) > 0 {
			clone.Roles = append([]agent.TeamRoleDefinition(nil), team.Roles...)
		}
		if len(team.Stages) > 0 {
			clone.Stages = append([]agent.TeamStageDefinition(nil), team.Stages...)
		}
		out[i] = clone
	}
	return out
}

// TeamRunRecorder returns the configured team run recorder.
func (c *AgentCoordinator) TeamRunRecorder() agent.TeamRunRecorder {
	if c == nil {
		return nil
	}
	return c.teamRunRecorder
}

// ReplyBackgroundInput routes a response to the manager that owns taskID.
func (c *AgentCoordinator) ReplyBackgroundInput(ctx context.Context, resp agent.InputResponse) error {
	if c == nil {
		return fmt.Errorf("agent coordinator is nil")
	}
	if c.bgRegistry == nil {
		return fmt.Errorf("background task registry not available")
	}

	taskID := strings.TrimSpace(resp.TaskID)
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	resp.TaskID = taskID

	sessionID, mgr, err := c.bgRegistry.findTaskManager(taskID)
	if err != nil {
		return err
	}
	if err := mgr.ReplyExternalInput(ctx, resp); err != nil {
		return err
	}
	c.bgRegistry.touch(sessionID)
	return nil
}

// InjectBackgroundInput routes free-form input to the manager that owns taskID.
func (c *AgentCoordinator) InjectBackgroundInput(ctx context.Context, taskID, input string) error {
	if c == nil {
		return fmt.Errorf("agent coordinator is nil")
	}
	if c.bgRegistry == nil {
		return fmt.Errorf("background task registry not available")
	}

	taskID = strings.TrimSpace(taskID)
	input = strings.TrimSpace(input)
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if input == "" {
		return fmt.Errorf("message is required")
	}

	sessionID, mgr, err := c.bgRegistry.findTaskManager(taskID)
	if err != nil {
		return err
	}
	if err := mgr.InjectBackgroundInput(ctx, taskID, input); err != nil {
		return err
	}
	c.bgRegistry.touch(sessionID)
	return nil
}
