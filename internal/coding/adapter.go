package coding

import (
	"context"
	"errors"
)

// ErrNotSupported indicates the adapter does not support the requested operation.
var ErrNotSupported = errors.New("operation not supported")

// Adapter defines the implementation contract for coding agent adapters.
type Adapter interface {
	Name() string
	Submit(ctx context.Context, req TaskRequest) (*TaskResult, error)
	Stream(ctx context.Context, req TaskRequest, cb ProgressCallback) (*TaskResult, error)
	Cancel(ctx context.Context, taskID string) error
	Status(ctx context.Context, taskID string) (TaskStatus, error)
}
