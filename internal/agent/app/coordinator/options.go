package coordinator

import (
	"alex/internal/agent/app/cost"
	"alex/internal/agent/app/hooks"
	"alex/internal/agent/app/preparation"
	react "alex/internal/agent/domain/react"
	agent "alex/internal/agent/ports/agent"
)

// CoordinatorOption configures optional dependencies for the agent coordinator.
type CoordinatorOption func(*AgentCoordinator)

// WithLogger overrides the default coordinator logger.
func WithLogger(logger agent.Logger) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithClock overrides the default coordinator clock.
func WithClock(clock agent.Clock) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if clock != nil {
			c.clock = clock
		}
	}
}

// WithCostTrackingDecorator overrides the default cost tracking decorator.
// This allows injecting a custom decorator for testing or alternative cost tracking strategies.
func WithCostTrackingDecorator(decorator *cost.CostTrackingDecorator) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if decorator != nil {
			c.costDecorator = decorator
		}
	}
}

// WithHookRegistry sets the proactive hook registry for pre/post-task processing.
func WithHookRegistry(registry *hooks.Registry) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if registry != nil {
			c.hookRegistry = registry
		}
	}
}

// WithExternalExecutor provides the external agent executor registry.
func WithExternalExecutor(executor agent.ExternalAgentExecutor) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if executor != nil {
			c.externalExecutor = executor
		}
	}
}

// WithIterationHook provides an iteration hook for mid-loop behavior.
func WithIterationHook(hook agent.IterationHook) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if hook != nil {
			c.iterationHook = hook
		}
	}
}

// WithCheckpointStore provides a checkpoint store for ReAct recovery.
func WithCheckpointStore(store react.CheckpointStore) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if store != nil {
			c.checkpointStore = store
		}
	}
}

// WithOKRContextProvider provides the OKR context provider for system prompt injection.
func WithOKRContextProvider(provider preparation.OKRContextProvider) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if provider != nil {
			c.okrContextProvider = provider
		}
	}
}

// WithCredentialRefresher provides a function that re-resolves CLI credentials
// at task execution time. This keeps long-running servers (e.g. Lark) working
// when startup tokens expire and need OAuth refresh.
func WithCredentialRefresher(fn preparation.CredentialRefresher) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if fn != nil {
			c.credentialRefresher = fn
		}
	}
}
