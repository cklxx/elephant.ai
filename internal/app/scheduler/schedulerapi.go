// Package scheduler — schedulerapi.go defines the service interface and DTO types
// shared between the scheduler implementation and the scheduler tools.
package scheduler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/robfig/cron/v3"
)

// Compile-time check that *Scheduler satisfies Service.
var _ Service = (*Scheduler)(nil)

// JobDTO is a Data Transfer Object for job data exposed to external consumers.
// The scheduler converts its internal Job type to this DTO when returning
// results through the Service interface.
type JobDTO struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	CronExpr     string          `json:"cron_expr"`
	Trigger      string          `json:"trigger"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Status       string          `json:"status"`
	LastRun      time.Time       `json:"last_run,omitempty"`
	NextRun      time.Time       `json:"next_run,omitempty"`
	FailureCount int             `json:"failure_count,omitempty"`
	LastFailure  time.Time       `json:"last_failure,omitempty"`
	LastError    string          `json:"last_error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// Service is the contract that the scheduler tools use to interact with the
// scheduler subsystem. The *Scheduler type satisfies this interface.
type Service interface {
	// RegisterDynamicTrigger creates and schedules a new job, returning its
	// persisted representation as a DTO.
	RegisterDynamicTrigger(ctx context.Context, name, schedule, task, channel string) (*JobDTO, error)
	// UnregisterTrigger removes a job by name from the scheduler and store.
	UnregisterTrigger(ctx context.Context, name string) error
	// ListJobs returns all persisted jobs as DTOs.
	ListJobs(ctx context.Context) ([]JobDTO, error)
	// LoadJob loads a single job by ID.
	LoadJob(ctx context.Context, id string) (*JobDTO, error)
	// CronParser returns the parser for validating cron expressions.
	CronParser() cron.Parser
}
