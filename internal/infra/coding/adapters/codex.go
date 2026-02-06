package adapters

import (
	"context"
	"fmt"

	"alex/internal/infra/coding"
	"alex/internal/infra/external/codex"
)

// CodexAdapter wraps the Codex external executor.
type CodexAdapter struct {
	executor *codex.Executor
}

// NewCodexAdapter constructs a Codex adapter.
func NewCodexAdapter(executor *codex.Executor) *CodexAdapter {
	return &CodexAdapter{executor: executor}
}

// Name returns adapter name.
func (a *CodexAdapter) Name() string {
	return "codex"
}

// Submit executes a coding task.
func (a *CodexAdapter) Submit(ctx context.Context, req coding.TaskRequest) (*coding.TaskResult, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("codex executor not configured")
	}
	result, err := a.executor.Execute(ctx, toExternalRequest(req, nil))
	if err != nil {
		return nil, err
	}
	return toTaskResult(req.TaskID, result), nil
}

// Stream executes a coding task with progress callback.
func (a *CodexAdapter) Stream(ctx context.Context, req coding.TaskRequest, cb coding.ProgressCallback) (*coding.TaskResult, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("codex executor not configured")
	}
	progress := wrapProgress(cb)
	result, err := a.executor.Execute(ctx, toExternalRequest(req, progress))
	if err != nil {
		return nil, err
	}
	return toTaskResult(req.TaskID, result), nil
}

// Cancel is not supported.
func (a *CodexAdapter) Cancel(_ context.Context, _ string) error {
	return coding.ErrNotSupported
}

// Status is not supported.
func (a *CodexAdapter) Status(_ context.Context, _ string) (coding.TaskStatus, error) {
	return coding.TaskStatus{}, coding.ErrNotSupported
}
