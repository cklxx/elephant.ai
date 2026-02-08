package devops

import (
	"context"
	"fmt"
	"time"
)

// ServiceState represents the lifecycle state of a managed service.
type ServiceState int

const (
	StateStopped  ServiceState = iota
	StateStarting
	StateRunning
	StateHealthy
	StateStopping
	StateFailed
)

func (s ServiceState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateHealthy:
		return "healthy"
	case StateStopping:
		return "stopping"
	case StateFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// HealthResult captures the outcome of a health probe.
type HealthResult struct {
	Healthy bool
	Message string
	Latency time.Duration
}

// Service represents a managed development service.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	State() ServiceState
	Health(ctx context.Context) HealthResult
}
