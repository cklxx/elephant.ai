package app

import (
	"context"
	"sync"

	"alex/internal/app/di"
	"alex/internal/delivery/server/ports"
)

// HealthCheckerImpl aggregates health probes for all components
type HealthCheckerImpl struct {
	probes []ports.HealthProbe
	mu     sync.RWMutex
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthCheckerImpl {
	return &HealthCheckerImpl{
		probes: make([]ports.HealthProbe, 0),
	}
}

// RegisterProbe adds a health probe
func (h *HealthCheckerImpl) RegisterProbe(probe ports.HealthProbe) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.probes = append(h.probes, probe)
}

// CheckAll returns health status for all components
func (h *HealthCheckerImpl) CheckAll(ctx context.Context) []ports.ComponentHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make([]ports.ComponentHealth, 0, len(h.probes))
	for _, probe := range h.probes {
		results = append(results, probe.Check(ctx))
	}
	return results
}

// DegradedComponentsSource provides a snapshot of degraded components.
// Satisfied by bootstrap.DegradedComponents.
type DegradedComponentsSource interface {
	Map() map[string]string
	IsEmpty() bool
}

// DegradedProbe reports bootstrap components that failed optional initialization.
type DegradedProbe struct {
	source DegradedComponentsSource
}

// NewDegradedProbe creates a probe that reports degraded bootstrap components.
func NewDegradedProbe(source DegradedComponentsSource) *DegradedProbe {
	return &DegradedProbe{source: source}
}

// Check returns one ComponentHealth entry per degraded component.
// If nothing is degraded, returns a single "ready" entry.
func (p *DegradedProbe) Check(ctx context.Context) ports.ComponentHealth {
	if p.source == nil || p.source.IsEmpty() {
		return ports.ComponentHealth{
			Name:    "bootstrap",
			Status:  ports.HealthStatusReady,
			Message: "All optional components initialized",
		}
	}
	degraded := p.source.Map()
	return ports.ComponentHealth{
		Name:    "bootstrap",
		Status:  ports.HealthStatusNotReady,
		Message: "Some optional components failed to initialize",
		Details: degraded,
	}
}

// ModelHealthFunc returns per-model health data as an opaque slice for the Details field.
type ModelHealthFunc func() interface{}

// LLMModelHealthProbe reports per-model health via the health endpoint.
type LLMModelHealthProbe struct {
	fn ModelHealthFunc
}

// NewLLMModelHealthProbe creates a probe that exposes per-model health data.
func NewLLMModelHealthProbe(fn ModelHealthFunc) *LLMModelHealthProbe {
	return &LLMModelHealthProbe{fn: fn}
}

// Check returns per-model health snapshots in the Details field.
func (p *LLMModelHealthProbe) Check(ctx context.Context) ports.ComponentHealth {
	if p.fn == nil {
		return ports.ComponentHealth{
			Name:    "llm_models",
			Status:  ports.HealthStatusDisabled,
			Message: "Model health tracking not enabled",
		}
	}

	details := p.fn()
	if details == nil {
		return ports.ComponentHealth{
			Name:    "llm_models",
			Status:  ports.HealthStatusReady,
			Message: "No models tracked yet",
		}
	}

	return ports.ComponentHealth{
		Name:    "llm_models",
		Status:  ports.HealthStatusReady,
		Message: "Per-model health data",
		Details: details,
	}
}

// LLMFactoryProbe checks LLM factory health
type LLMFactoryProbe struct {
	container *di.Container
}

// NewLLMFactoryProbe creates a new LLM factory health probe
func NewLLMFactoryProbe(container *di.Container) *LLMFactoryProbe {
	return &LLMFactoryProbe{
		container: container,
	}
}

// Check returns the health status of LLM factory
func (p *LLMFactoryProbe) Check(ctx context.Context) ports.ComponentHealth {
	if p == nil || p.container == nil {
		return ports.ComponentHealth{
			Name:    "llm_factory",
			Status:  ports.HealthStatusNotReady,
			Message: "LLM factory container not initialized",
		}
	}
	if !p.container.HasLLMFactory() {
		return ports.ComponentHealth{
			Name:    "llm_factory",
			Status:  ports.HealthStatusNotReady,
			Message: "LLM factory not initialized",
		}
	}
	return ports.ComponentHealth{
		Name:    "llm_factory",
		Status:  ports.HealthStatusReady,
		Message: "LLM factory initialized",
	}
}
