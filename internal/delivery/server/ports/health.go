package ports

import "context"

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusReady    HealthStatus = "ready"
	HealthStatusNotReady HealthStatus = "not_ready"
	HealthStatusDisabled HealthStatus = "disabled"
	HealthStatusError    HealthStatus = "error"
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Name    string       `json:"name"`
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
	Details interface{}  `json:"details,omitempty"`
}

// HealthProbe checks the health of a component
type HealthProbe interface {
	// Check returns the health status of the component
	Check(ctx context.Context) ComponentHealth
}

// HealthChecker aggregates multiple health probes
type HealthChecker interface {
	// CheckAll returns health status for all components
	CheckAll(ctx context.Context) []ComponentHealth

	// RegisterProbe adds a health probe
	RegisterProbe(probe HealthProbe)
}

// ModelHealthProvider supplies pre-processed model health data.
// Methods use only builtin types so implementers (e.g. app/di.Container) satisfy
// the interface structurally without importing this package.
type ModelHealthProvider interface {
	// AggregateModelHealth returns (healthy, message).
	// healthy is false when any model is degraded or down.
	AggregateModelHealth() (bool, string)
	// SanitizedModelHealth returns per-model data safe for external exposure.
	// Returns nil when no models are tracked.
	SanitizedModelHealth() interface{}
}
