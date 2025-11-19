package app

import (
	"alex/internal/agent/ports"
)

// CoordinatorOption configures optional dependencies for the agent coordinator.
type CoordinatorOption func(*AgentCoordinator)

// WithLogger overrides the default coordinator logger.
func WithLogger(logger ports.Logger) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithClock overrides the default coordinator clock.
func WithClock(clock ports.Clock) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if clock != nil {
			c.clock = clock
		}
	}
}

// WithTaskAnalysisService overrides the default task analysis service.
// This allows injecting a custom analysis service for testing or alternative analysis strategies.
func WithTaskAnalysisService(service *TaskAnalysisService) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if service != nil {
			c.analysisService = service
		}
	}
}

// WithCostTrackingDecorator overrides the default cost tracking decorator.
// This allows injecting a custom decorator for testing or alternative cost tracking strategies.
func WithCostTrackingDecorator(decorator *CostTrackingDecorator) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if decorator != nil {
			c.costDecorator = decorator
		}
	}
}
