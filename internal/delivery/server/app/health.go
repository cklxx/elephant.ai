package app

import (
	"context"
	"fmt"
	"sync"

	"alex/internal/app/di"
	"alex/internal/delivery/server/ports"
	"alex/internal/infra/llm"
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

// ModelHealthDetails returns per-model sanitized health data for the debug endpoint.
// Returns nil if no LLMModelHealthProbe is registered.
func (h *HealthCheckerImpl) ModelHealthDetails() interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, probe := range h.probes {
		if mp, ok := probe.(*LLMModelHealthProbe); ok {
			return mp.DetailedHealth()
		}
	}
	return nil
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

// LLMModelHealthProbe reports aggregate LLM health via the public /health endpoint.
// Per-model telemetry (error rates, latency percentiles) is only available through
// the debug endpoint /api/debug/health/models.
type LLMModelHealthProbe struct {
	fn ModelHealthFunc
}

// NewLLMModelHealthProbe creates a probe that exposes aggregate model health.
func NewLLMModelHealthProbe(fn ModelHealthFunc) *LLMModelHealthProbe {
	return &LLMModelHealthProbe{fn: fn}
}

// Check returns an aggregate health summary without per-model telemetry.
// Only the model count and overall state are exposed on the public /health endpoint.
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

	aggregate := aggregateModelHealth(details)
	return ports.ComponentHealth{
		Name:    "llm_models",
		Status:  aggregate.status,
		Message: aggregate.message,
	}
}

// DetailedHealth returns per-model sanitized health data for the debug endpoint.
func (p *LLMModelHealthProbe) DetailedHealth() interface{} {
	if p.fn == nil {
		return nil
	}
	raw := p.fn()
	return sanitizeModelHealth(raw)
}

// modelAggregate holds the computed aggregate across all tracked models.
type modelAggregate struct {
	status  ports.HealthStatus
	message string
}

// aggregateModelHealth reduces per-model data to a single aggregate status.
func aggregateModelHealth(raw interface{}) modelAggregate {
	healths, ok := raw.([]llm.ProviderHealth)
	if !ok || len(healths) == 0 {
		return modelAggregate{
			status:  ports.HealthStatusReady,
			message: "No models tracked yet",
		}
	}

	var totalScore float64
	var downCount, degradedCount int
	for _, h := range healths {
		totalScore += h.HealthScore
		switch string(h.State) {
		case "down":
			downCount++
		case "degraded":
			degradedCount++
		}
	}
	avgScore := totalScore / float64(len(healths))

	status := ports.HealthStatusReady
	if downCount > 0 {
		status = ports.HealthStatusNotReady
	} else if degradedCount > 0 {
		status = ports.HealthStatusNotReady
	}

	return modelAggregate{
		status:  status,
		message: fmt.Sprintf("%d models tracked, avg health score %.0f", len(healths), avgScore),
	}
}

// sanitizeModelHealth converts internal ProviderHealth data to the external-safe
// SanitizedHealth form. Handles both []ProviderHealth and unknown types gracefully.
func sanitizeModelHealth(raw interface{}) interface{} {
	switch v := raw.(type) {
	case []llm.ProviderHealth:
		return llm.SanitizeAll(v)
	default:
		return nil
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
