package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"alex/internal/delivery/server/ports"
	agent "alex/internal/domain/agent/ports/agent"
	taskdomain "alex/internal/domain/task"
	id "alex/internal/shared/utils/id"
)

// ServerAdapter implements ports.TaskStore by delegating to the unified task store.
// It also implements the optional sessionTaskSummaryStore interface for batch
// session task summarisation.
type ServerAdapter struct {
	store taskdomain.Store
}

var _ ports.TaskStore = (*ServerAdapter)(nil)

// NewServerAdapter wraps a unified task store to satisfy the server's TaskStore port.
func NewServerAdapter(store taskdomain.Store) *ServerAdapter {
	return &ServerAdapter{store: store}
}

// Create creates a new task with optional presets.
func (a *ServerAdapter) Create(ctx context.Context, sessionID string, description string, agentPreset string, toolPreset string) (*ports.Task, error) {
	taskID := id.RunIDFromContext(ctx)
	if taskID == "" {
		taskID = id.NewRunID()
	}

	now := time.Now()
	t := &taskdomain.Task{
		TaskID:       taskID,
		SessionID:    sessionID,
		ParentTaskID: id.ParentRunIDFromContext(ctx),
		Channel:      "web",
		Description:  description,
		AgentPreset:  agentPreset,
		ToolPreset:   toolPreset,
		Status:       taskdomain.StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := a.store.Create(ctx, t); err != nil {
		return nil, err
	}

	return domainToServerTask(t), nil
}

// Get retrieves a task by ID.
func (a *ServerAdapter) Get(ctx context.Context, taskID string) (*ports.Task, error) {
	t, err := a.store.Get(ctx, taskID)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("task %s: not found", taskID)
		}
		return nil, err
	}
	return domainToServerTask(t), nil
}

// Update updates task state.
func (a *ServerAdapter) Update(ctx context.Context, task *ports.Task) error {
	return a.store.SetStatus(ctx, task.ID, serverStatusToDomain(task.Status),
		taskdomain.WithTransitionReason("update"),
	)
}

// List returns tasks with pagination.
func (a *ServerAdapter) List(ctx context.Context, limit int, offset int) ([]*ports.Task, int, error) {
	tasks, total, err := a.store.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*ports.Task, len(tasks))
	for i, t := range tasks {
		result[i] = domainToServerTask(t)
	}
	return result, total, nil
}

// ListBySession returns tasks for a specific session.
func (a *ServerAdapter) ListBySession(ctx context.Context, sessionID string) ([]*ports.Task, error) {
	tasks, err := a.store.ListBySession(ctx, sessionID, 0)
	if err != nil {
		return nil, err
	}
	result := make([]*ports.Task, len(tasks))
	for i, t := range tasks {
		result[i] = domainToServerTask(t)
	}
	return result, nil
}

// ListByStatus returns tasks filtered by one or more statuses.
func (a *ServerAdapter) ListByStatus(ctx context.Context, statuses ...ports.TaskStatus) ([]*ports.Task, error) {
	domainStatuses := make([]taskdomain.Status, len(statuses))
	for i, s := range statuses {
		domainStatuses[i] = serverStatusToDomain(s)
	}

	tasks, err := a.store.ListByStatus(ctx, domainStatuses...)
	if err != nil {
		return nil, err
	}
	result := make([]*ports.Task, len(tasks))
	for i, t := range tasks {
		result[i] = domainToServerTask(t)
	}
	return result, nil
}

// Delete removes a task.
func (a *ServerAdapter) Delete(ctx context.Context, taskID string) error {
	return a.store.Delete(ctx, taskID)
}

// SetStatus updates task status.
func (a *ServerAdapter) SetStatus(ctx context.Context, taskID string, status ports.TaskStatus) error {
	return a.store.SetStatus(ctx, taskID, serverStatusToDomain(status))
}

// SetError records task failure.
func (a *ServerAdapter) SetError(ctx context.Context, taskID string, err error) error {
	return a.store.SetError(ctx, taskID, err.Error())
}

// SetResult stores task completion result.
func (a *ServerAdapter) SetResult(ctx context.Context, taskID string, result *agent.TaskResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return a.store.SetResult(ctx, taskID, result.Answer, resultJSON, result.TokensUsed)
}

// UpdateProgress updates task execution progress.
func (a *ServerAdapter) UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int) error {
	return a.store.UpdateProgress(ctx, taskID, iteration, tokensUsed, 0)
}

// SetTerminationReason sets the termination reason for a task.
func (a *ServerAdapter) SetTerminationReason(ctx context.Context, taskID string, reason ports.TerminationReason) error {
	domainReason := serverTermToDomain(reason)
	return a.store.SetStatus(ctx, taskID, terminationToStatus(domainReason),
		taskdomain.WithTransitionReason(string(reason)),
	)
}

// TryClaimTask attempts to claim ownership for task execution.
func (a *ServerAdapter) TryClaimTask(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	return a.store.TryClaimTask(ctx, taskID, ownerID, leaseUntil)
}

// ClaimResumableTasks atomically claims tasks in statuses for resume execution.
func (a *ServerAdapter) ClaimResumableTasks(ctx context.Context, ownerID string, leaseUntil time.Time, limit int, statuses ...ports.TaskStatus) ([]*ports.Task, error) {
	domainStatuses := make([]taskdomain.Status, len(statuses))
	for i, s := range statuses {
		domainStatuses[i] = serverStatusToDomain(s)
	}
	tasks, err := a.store.ClaimResumableTasks(ctx, ownerID, leaseUntil, limit, domainStatuses...)
	if err != nil {
		return nil, err
	}
	result := make([]*ports.Task, len(tasks))
	for i, t := range tasks {
		result[i] = domainToServerTask(t)
	}
	return result, nil
}

// RenewTaskLease refreshes the task lease for an owner.
func (a *ServerAdapter) RenewTaskLease(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	return a.store.RenewTaskLease(ctx, taskID, ownerID, leaseUntil)
}

// ReleaseTaskLease releases task ownership for an owner.
func (a *ServerAdapter) ReleaseTaskLease(ctx context.Context, taskID, ownerID string) error {
	return a.store.ReleaseTaskLease(ctx, taskID, ownerID)
}

// Close is a no-op; the underlying Postgres pool is managed elsewhere.
func (a *ServerAdapter) Close() {}

// ── Conversion helpers ──────────────────────────────────────────────────────

func domainToServerTask(t *taskdomain.Task) *ports.Task {
	pt := &ports.Task{
		ID:                t.TaskID,
		SessionID:         t.SessionID,
		ParentTaskID:      t.ParentTaskID,
		Status:            domainStatusToServer(t.Status),
		Description:       t.Description,
		CreatedAt:         t.CreatedAt,
		StartedAt:         t.StartedAt,
		CompletedAt:       t.CompletedAt,
		Error:             t.Error,
		TerminationReason: domainTermToServer(t.TerminationReason),
		CurrentIteration:  t.CurrentIteration,
		TotalIterations:   t.TotalIterations,
		TokensUsed:        t.TokensUsed,
		TotalTokens:       t.TokensUsed,
		Metadata:          t.Metadata,
		AgentPreset:       t.AgentPreset,
		ToolPreset:        t.ToolPreset,
	}

	if t.ResultJSON != nil {
		var result agent.TaskResult
		if json.Unmarshal(t.ResultJSON, &result) == nil {
			pt.Result = &result
		}
	}

	return pt
}

func domainStatusToServer(s taskdomain.Status) ports.TaskStatus {
	switch s {
	case taskdomain.StatusPending:
		return ports.TaskStatusPending
	case taskdomain.StatusRunning, taskdomain.StatusWaitingInput:
		return ports.TaskStatusRunning
	case taskdomain.StatusCompleted:
		return ports.TaskStatusCompleted
	case taskdomain.StatusFailed:
		return ports.TaskStatusFailed
	case taskdomain.StatusCancelled:
		return ports.TaskStatusCancelled
	default:
		return ports.TaskStatus(s)
	}
}

func serverStatusToDomain(s ports.TaskStatus) taskdomain.Status {
	switch s {
	case ports.TaskStatusPending:
		return taskdomain.StatusPending
	case ports.TaskStatusRunning:
		return taskdomain.StatusRunning
	case ports.TaskStatusCompleted:
		return taskdomain.StatusCompleted
	case ports.TaskStatusFailed:
		return taskdomain.StatusFailed
	case ports.TaskStatusCancelled:
		return taskdomain.StatusCancelled
	default:
		return taskdomain.Status(s)
	}
}

func domainTermToServer(r taskdomain.TerminationReason) ports.TerminationReason {
	switch r {
	case taskdomain.TerminationCompleted:
		return ports.TerminationReasonCompleted
	case taskdomain.TerminationCancelled:
		return ports.TerminationReasonCancelled
	case taskdomain.TerminationTimeout:
		return ports.TerminationReasonTimeout
	case taskdomain.TerminationError:
		return ports.TerminationReasonError
	default:
		return ports.TerminationReasonNone
	}
}

func serverTermToDomain(r ports.TerminationReason) taskdomain.TerminationReason {
	switch r {
	case ports.TerminationReasonCompleted:
		return taskdomain.TerminationCompleted
	case ports.TerminationReasonCancelled:
		return taskdomain.TerminationCancelled
	case ports.TerminationReasonTimeout:
		return taskdomain.TerminationTimeout
	case ports.TerminationReasonError:
		return taskdomain.TerminationError
	default:
		return taskdomain.TerminationNone
	}
}

func terminationToStatus(r taskdomain.TerminationReason) taskdomain.Status {
	switch r {
	case taskdomain.TerminationCompleted:
		return taskdomain.StatusCompleted
	case taskdomain.TerminationCancelled:
		return taskdomain.StatusCancelled
	case taskdomain.TerminationTimeout, taskdomain.TerminationError:
		return taskdomain.StatusFailed
	default:
		return taskdomain.StatusPending
	}
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
