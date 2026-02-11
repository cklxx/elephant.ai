package external

import (
	"context"
	"fmt"
	"strings"
	"sync"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/external/bridge"
	"alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// Registry routes external agent requests to the appropriate executor.
type Registry struct {
	executors map[string]agent.ExternalAgentExecutor
	inputCh   chan agent.InputRequest
	pending   sync.Map
	logger    logging.Logger
}

// NewRegistry constructs a registry from runtime external agent config.
func NewRegistry(cfg config.ExternalAgentsConfig, logger logging.Logger) *Registry {
	registry := &Registry{
		executors: make(map[string]agent.ExternalAgentExecutor),
		logger:    logging.OrNop(logger),
	}

	if cfg.ClaudeCode.Enabled {
		exec := bridge.New(bridge.BridgeConfig{
			AgentType:              "claude_code",
			Binary:                 cfg.ClaudeCode.Binary,
			Interactive:            true,
			APIKey:                 cfg.ClaudeCode.Env["ANTHROPIC_API_KEY"],
			DefaultModel:           cfg.ClaudeCode.DefaultModel,
			DefaultMode:            cfg.ClaudeCode.DefaultMode,
			AutonomousAllowedTools: cfg.ClaudeCode.AutonomousAllowedTools,
			MaxBudgetUSD:           cfg.ClaudeCode.MaxBudgetUSD,
			MaxTurns:               cfg.ClaudeCode.MaxTurns,
			Timeout:                cfg.ClaudeCode.Timeout,
			Env:                    cfg.ClaudeCode.Env,
		})
		registry.register(exec)
	}
	if cfg.Codex.Enabled {
		exec := bridge.New(bridge.BridgeConfig{
			AgentType:      "codex",
			Binary:         cfg.Codex.Binary,
			Interactive:    false,
			APIKey:         cfg.Codex.Env["OPENAI_API_KEY"],
			DefaultModel:   cfg.Codex.DefaultModel,
			ApprovalPolicy: cfg.Codex.ApprovalPolicy,
			Sandbox:        cfg.Codex.Sandbox,
			Timeout:        cfg.Codex.Timeout,
			Env:            cfg.Codex.Env,
		})
		registry.register(exec)
	}

	return registry
}

func (r *Registry) register(exec agent.ExternalAgentExecutor) {
	if exec == nil {
		return
	}
	for _, agentType := range exec.SupportedTypes() {
		if strings.TrimSpace(agentType) == "" {
			continue
		}
		r.executors[agentType] = exec
	}
	if interactive, ok := exec.(agent.InteractiveExternalExecutor); ok {
		if r.inputCh == nil {
			r.inputCh = make(chan agent.InputRequest, 64)
		}
		go r.forwardRequests(interactive)
	}
}

func (r *Registry) forwardRequests(exec agent.InteractiveExternalExecutor) {
	for req := range exec.InputRequests() {
		key := requestKey(req.TaskID, req.RequestID)
		r.pending.Store(key, exec)
		select {
		case r.inputCh <- req:
		default:
			r.logger.Warn("external input channel full, dropping request %s", req.RequestID)
		}
	}
}

func (r *Registry) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if req.AgentType == "" {
		return nil, fmt.Errorf("agent_type is required for external execution")
	}
	exec, ok := r.executors[req.AgentType]
	if !ok {
		return nil, fmt.Errorf("unsupported external agent type: %s", req.AgentType)
	}
	return exec.Execute(ctx, req)
}

func (r *Registry) SupportedTypes() []string {
	out := make([]string, 0, len(r.executors))
	for key := range r.executors {
		out = append(out, key)
	}
	return out
}

func (r *Registry) InputRequests() <-chan agent.InputRequest {
	return r.inputCh
}

func (r *Registry) Reply(ctx context.Context, resp agent.InputResponse) error {
	key := requestKey(resp.TaskID, resp.RequestID)
	if execVal, ok := r.pending.Load(key); ok {
		exec := execVal.(agent.InteractiveExternalExecutor)
		err := exec.Reply(ctx, resp)
		if err == nil {
			r.pending.Delete(key)
		}
		return err
	}
	return fmt.Errorf("unknown request_id: %s", resp.RequestID)
}

func requestKey(taskID, requestID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(taskID), strings.TrimSpace(requestID))
}
