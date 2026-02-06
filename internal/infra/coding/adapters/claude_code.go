package adapters

import (
	"context"
	"fmt"

	"alex/internal/infra/coding"
	"alex/internal/infra/external/claudecode"
)

// ClaudeCodeAdapter wraps the Claude Code executor.
type ClaudeCodeAdapter struct {
	executor *claudecode.Executor
}

// NewClaudeCodeAdapter constructs a Claude Code adapter.
func NewClaudeCodeAdapter(executor *claudecode.Executor) *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{executor: executor}
}

// Name returns adapter name.
func (a *ClaudeCodeAdapter) Name() string {
	return "claude_code"
}

// Submit executes a coding task.
func (a *ClaudeCodeAdapter) Submit(ctx context.Context, req coding.TaskRequest) (*coding.TaskResult, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("claude code executor not configured")
	}
	result, err := a.executor.Execute(ctx, toExternalRequest(req, nil))
	if err != nil {
		return nil, err
	}
	return toTaskResult(req.TaskID, result), nil
}

// Stream executes a coding task with progress callback.
func (a *ClaudeCodeAdapter) Stream(ctx context.Context, req coding.TaskRequest, cb coding.ProgressCallback) (*coding.TaskResult, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("claude code executor not configured")
	}
	progress := wrapProgress(cb)
	result, err := a.executor.Execute(ctx, toExternalRequest(req, progress))
	if err != nil {
		return nil, err
	}
	return toTaskResult(req.TaskID, result), nil
}

// Cancel is not supported.
func (a *ClaudeCodeAdapter) Cancel(_ context.Context, _ string) error {
	return coding.ErrNotSupported
}

// Status is not supported.
func (a *ClaudeCodeAdapter) Status(_ context.Context, _ string) (coding.TaskStatus, error) {
	return coding.TaskStatus{}, coding.ErrNotSupported
}
