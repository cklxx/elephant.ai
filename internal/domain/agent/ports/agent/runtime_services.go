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

// LatencyReporterFunc emits runtime latency diagnostics.
type LatencyReporterFunc func(ctx context.Context, format string, args ...any)

// JSONMarshalFunc serializes values used in runtime prompts.
type JSONMarshalFunc func(v any) ([]byte, error)

// GoRunnerFunc executes asynchronous work with a named task label.
type GoRunnerFunc func(logger Logger, name string, fn func())

// WorkingDirResolverFunc resolves the execution working directory from context.
type WorkingDirResolverFunc func(ctx context.Context) string

// WorkspaceManager allocates and merges isolated execution workspaces.
type WorkspaceManager interface {
	Allocate(ctx context.Context, taskID string, mode WorkspaceMode, fileScope []string) (*WorkspaceAllocation, error)
	Merge(ctx context.Context, alloc *WorkspaceAllocation, strategy MergeStrategy) (*MergeResult, error)
}

// WorkspaceManagerFactoryFunc constructs workspace managers for a working directory.
type WorkspaceManagerFactoryFunc func(workingDir string, logger Logger) WorkspaceManager
