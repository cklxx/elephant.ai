package coding

import (
	"context"
	"fmt"
)

// Gateway exposes a unified interface for coding agents.
type Gateway interface {
	Submit(ctx context.Context, req TaskRequest) (*TaskResult, error)
	Stream(ctx context.Context, req TaskRequest, cb ProgressCallback) (*TaskResult, error)
	Cancel(ctx context.Context, taskID string) error
	Status(ctx context.Context, taskID string) (TaskStatus, error)
}

// defaultGateway routes requests to registered adapters.
type defaultGateway struct {
	registry       *adapterRegistry
	defaultAdapter string
}

// NewGateway constructs a defaultGateway.
func NewGateway(registry *adapterRegistry, defaultAdapter string) *defaultGateway {
	if registry == nil {
		registry = NewAdapterRegistry()
	}
	return &defaultGateway{registry: registry, defaultAdapter: defaultAdapter}
}

// Submit dispatches a task to the selected adapter.
func (g *defaultGateway) Submit(ctx context.Context, req TaskRequest) (*TaskResult, error) {
	adapter, err := g.selectAdapter(req)
	if err != nil {
		return nil, err
	}
	return adapter.Submit(ctx, req)
}

// Stream dispatches a task with progress callback.
func (g *defaultGateway) Stream(ctx context.Context, req TaskRequest, cb ProgressCallback) (*TaskResult, error) {
	adapter, err := g.selectAdapter(req)
	if err != nil {
		return nil, err
	}
	if cb == nil {
		return adapter.Submit(ctx, req)
	}
	return adapter.Stream(ctx, req, cb)
}

// Cancel forwards cancellation to the adapter if supported.
func (g *defaultGateway) Cancel(ctx context.Context, taskID string) error {
	adapter, err := g.defaultAdapterForCancel()
	if err != nil {
		return err
	}
	return adapter.Cancel(ctx, taskID)
}

// Status returns the status from the adapter if supported.
func (g *defaultGateway) Status(ctx context.Context, taskID string) (TaskStatus, error) {
	adapter, err := g.defaultAdapterForCancel()
	if err != nil {
		return TaskStatus{}, err
	}
	return adapter.Status(ctx, taskID)
}

func (g *defaultGateway) selectAdapter(req TaskRequest) (Adapter, error) {
	if g == nil || g.registry == nil {
		return nil, fmt.Errorf("coding gateway not initialized")
	}
	if req.AgentType != "" {
		return g.registry.Get(req.AgentType)
	}
	if g.defaultAdapter != "" {
		return g.registry.Get(g.defaultAdapter)
	}
	adapters := g.registry.List()
	if len(adapters) == 1 {
		return adapters[0], nil
	}
	if len(adapters) == 0 {
		return nil, fmt.Errorf("no adapters registered")
	}
	return nil, fmt.Errorf("multiple adapters available; agent_type required")
}

func (g *defaultGateway) defaultAdapterForCancel() (Adapter, error) {
	if g == nil || g.registry == nil {
		return nil, fmt.Errorf("coding gateway not initialized")
	}
	if g.defaultAdapter != "" {
		return g.registry.Get(g.defaultAdapter)
	}
	adapters := g.registry.List()
	if len(adapters) == 1 {
		return adapters[0], nil
	}
	return nil, fmt.Errorf("default adapter not configured")
}
