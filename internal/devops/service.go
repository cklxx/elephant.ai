package devops

import (
	"context"
	"fmt"

	"alex/internal/devops/health"
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

// Service represents a managed development service.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	State() ServiceState
	Health(ctx context.Context) health.Result
}

// Buildable is an optional interface for services that compile a binary.
// Orchestrator checks for this interface during Restart to build before
// stopping the old process, so a compilation failure never causes downtime.
type Buildable interface {
	// Build compiles to a staging path without touching the running binary.
	Build(ctx context.Context) (stagingPath string, err error)
	// Promote atomically replaces the production binary with the staged one.
	Promote(stagingPath string) error
}
