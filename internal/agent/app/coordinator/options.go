package coordinator

import (
	"alex/internal/agent/app/cost"
	"alex/internal/agent/app/hooks"
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
