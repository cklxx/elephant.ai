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

// KernelHealthProbe checks kernel engine liveness.
type KernelHealthProbe struct {
	container *di.Container
}

// NewKernelHealthProbe creates a kernel health probe.
func NewKernelHealthProbe(container *di.Container) *KernelHealthProbe {
	return &KernelHealthProbe{container: container}
}

// Check returns the health status of the kernel engine.
func (p *KernelHealthProbe) Check(_ context.Context) ports.ComponentHealth {
	if p.container == nil || p.container.KernelEngine == nil {
		return ports.ComponentHealth{
			Name:    "kernel",
			Status:  ports.HealthStatusDisabled,
			Message: "Kernel engine not initialized",
		}
	}
	h := p.container.KernelEngine.HealthStatus()
	details := map[string]interface{}{
		"loop_restarts":        h.LoopRestarts,
		"consecutive_failures": h.ConsecutiveFailures,
	}
	if !h.LastCycleAt.IsZero() {
		details["last_cycle_at"] = h.LastCycleAt.Format("2006-01-02T15:04:05Z")
	}
	if !h.LastSuccessAt.IsZero() {
		details["last_success_at"] = h.LastSuccessAt.Format("2006-01-02T15:04:05Z")
	}
	if h.Ready {
		return ports.ComponentHealth{
			Name:    "kernel",
			Status:  ports.HealthStatusReady,
			Message: h.Reason,
			Details: details,
		}
	}
	return ports.ComponentHealth{
		Name:    "kernel",
		Status:  ports.HealthStatusNotReady,
		Message: h.Reason,
		Details: details,
	}
}
