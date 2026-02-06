package agent

import "context"

// IDContextValues captures identifier values stored on a request context.
type IDContextValues struct {
	SessionID     string
	RunID         string
	ParentRunID   string
	LogID         string
	CorrelationID string
	CausationID   string
}

// IDGenerator creates stable identifiers used by the agent runtime.
type IDGenerator interface {
	NewEventID() string
	NewRunID() string
	NewRequestIDWithLogID(logID string) string
	NewLogID() string
	NewKSUID() string
	NewUUIDv7() string
}

// IDContextReader reads and writes correlation identifiers from context.
type IDContextReader interface {
	LogIDFromContext(ctx context.Context) string
	CorrelationIDFromContext(ctx context.Context) string
	CausationIDFromContext(ctx context.Context) string
	IDsFromContext(ctx context.Context) IDContextValues
	WithSessionID(ctx context.Context, sessionID string) context.Context
	WithRunID(ctx context.Context, runID string) context.Context
	WithParentRunID(ctx context.Context, parentRunID string) context.Context
	WithCorrelationID(ctx context.Context, correlationID string) context.Context
	WithCausationID(ctx context.Context, causationID string) context.Context
	WithLogID(ctx context.Context, logID string) context.Context
}

// LatencyReporter emits runtime latency diagnostics.
type LatencyReporter interface {
	PrintfWithContext(ctx context.Context, format string, args ...any)
}

// JSONCodec serializes values used in runtime prompts.
type JSONCodec interface {
	Marshal(v any) ([]byte, error)
}

// GoRunner executes asynchronous work with a named task label.
type GoRunner interface {
	Go(logger Logger, name string, fn func())
}

// WorkingDirResolver resolves the execution working directory from context.
type WorkingDirResolver interface {
	ResolveWorkingDir(ctx context.Context) string
}

// WorkspaceManager allocates and merges isolated execution workspaces.
type WorkspaceManager interface {
	Allocate(ctx context.Context, taskID string, mode WorkspaceMode, fileScope []string) (*WorkspaceAllocation, error)
	Merge(ctx context.Context, alloc *WorkspaceAllocation, strategy MergeStrategy) (*MergeResult, error)
}

// WorkspaceManagerFactory constructs workspace managers for a working directory.
type WorkspaceManagerFactory interface {
	NewWorkspaceManager(workingDir string, logger Logger) WorkspaceManager
}
