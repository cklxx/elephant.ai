package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// JobStatus represents the lifecycle state of a scheduled job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusActive    JobStatus = "active"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed"
)

// validJobStatuses enumerates all accepted status values.
var validJobStatuses = map[JobStatus]bool{
	JobStatusPending:   true,
	JobStatusActive:    true,
	JobStatusPaused:    true,
	JobStatusCompleted: true,
}

// IsValid returns true if the status is one of the recognized values.
func (s JobStatus) IsValid() bool {
	return validJobStatuses[s]
}

// Job represents a persistable scheduled job. It captures the schedule
// definition (CronExpr), the action to perform (Trigger + Payload), and
// lifecycle metadata (Status, LastRun, NextRun, timestamps).
type Job struct {
	// ID is the unique identifier for this job (typically a slug or UUID).
	ID string `json:"id"`
	// Name is a human-readable label for the job.
	Name string `json:"name"`
	// CronExpr is the cron schedule expression (5-field).
	CronExpr string `json:"cron_expr"`
	// Trigger describes what action the job performs (e.g. "okr_review",
	// "daily_briefing").
	Trigger string `json:"trigger"`
	// Payload holds arbitrary JSON-encodable parameters for the trigger.
	Payload json.RawMessage `json:"payload,omitempty"`
	// Status is the current lifecycle state of the job.
	Status JobStatus `json:"status"`
	// LastRun is the time the job last executed. Zero value means never run.
	LastRun time.Time `json:"last_run,omitempty"`
	// NextRun is the computed next execution time.
	NextRun time.Time `json:"next_run,omitempty"`
	// CreatedAt is when the job was first persisted.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the job was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that the job has the minimum required fields.
func (j *Job) Validate() error {
	if j.ID == "" {
		return fmt.Errorf("job: id is required")
	}
	if j.Name == "" {
		return fmt.Errorf("job: name is required")
	}
	if j.CronExpr == "" {
		return fmt.Errorf("job: cron_expr is required")
	}
	if !j.Status.IsValid() {
		return fmt.Errorf("job: invalid status %q", j.Status)
	}
	return nil
}

// ---------------------------------------------------------------------------
// JobStore defines the persistence contract for scheduled jobs.
// Implementations range from a filesystem store (FileJobStore) to database or
// object-storage backends.
// ---------------------------------------------------------------------------

// JobStore is the port through which the scheduler persists and retrieves
// job definitions.
type JobStore interface {
	// Save persists the job, creating or overwriting the entry for job.ID.
	Save(ctx context.Context, job Job) error
	// Load retrieves the job with the given ID. Returns a non-nil error
	// wrapping ErrJobNotFound if no job exists.
	Load(ctx context.Context, jobID string) (*Job, error)
	// List returns all persisted jobs.
	List(ctx context.Context) ([]Job, error)
	// Delete removes the job with the given ID. Returns a non-nil error
	// wrapping ErrJobNotFound if no job exists.
	Delete(ctx context.Context, jobID string) error
	// UpdateStatus transitions the job to the given status, updating the
	// UpdatedAt timestamp. Returns a non-nil error wrapping ErrJobNotFound
	// if no job exists.
	UpdateStatus(ctx context.Context, jobID string, status JobStatus) error
}

// ErrJobNotFound is returned when a job lookup fails because the requested
// ID does not exist in the store.
var ErrJobNotFound = fmt.Errorf("job not found")
