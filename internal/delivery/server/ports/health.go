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
