package shadow

import (
	"context"
	"fmt"
	"strings"

	tools "alex/internal/agent/ports/tools"
	"alex/internal/coding"
	"alex/internal/logging"
)

// Agent coordinates shadow agent execution with mandatory approvals.
type Agent struct {
	cfg      Config
	gateway  coding.Gateway
	approver tools.Approver
	logger   logging.Logger
}

// NewAgent constructs a shadow agent coordinator.
func NewAgent(cfg Config, gateway coding.Gateway, approver tools.Approver, logger logging.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		gateway:  gateway,
		approver: approver,
		logger:   logging.OrNop(logger),
	}
}

// Run executes the shadow task after mandatory approval.
func (a *Agent) Run(ctx context.Context, task Task) (*Result, error) {
	if a.gateway == nil {
		return nil, fmt.Errorf("coding gateway is required")
	}
	if err := RequireApproval(ctx, a.approver, task); err != nil {
		return nil, err
	}

	agentType := strings.TrimSpace(task.AgentType)
	if agentType == "" {
		agentType = strings.TrimSpace(a.cfg.DefaultAgentType)
	}

	request := coding.TaskRequest{
		TaskID:      task.ID,
		Prompt:      task.Prompt,
		AgentType:   agentType,
		WorkingDir:  task.WorkingDir,
		Config:      task.Config,
		SessionID:   task.SessionID,
		CausationID: task.CausationID,
	}

	result, err := a.gateway.Submit(ctx, request)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("shadow agent returned no result")
	}

	return &Result{
		TaskID:   task.ID,
		Answer:   result.Answer,
		Error:    result.Error,
		Metadata: result.Metadata,
	}, nil
}
