package runtime

import (
	"context"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/external/workspace"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/shared/async"
	"alex/internal/shared/json"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

// IDsAdapter bridges runtime context/id utilities into domain-facing ports.
type IDsAdapter struct{}

func (IDsAdapter) NewEventID() string                          { return id.NewEventID() }
func (IDsAdapter) NewRunID() string                            { return id.NewRunID() }
func (IDsAdapter) NewRequestIDWithLogID(logID string) string   { return id.NewRequestIDWithLogID(logID) }
func (IDsAdapter) NewLogID() string                            { return id.NewLogID() }
func (IDsAdapter) NewKSUID() string                            { return id.NewKSUID() }
func (IDsAdapter) NewUUIDv7() string                           { return id.NewUUIDv7() }
func (IDsAdapter) LogIDFromContext(ctx context.Context) string { return id.LogIDFromContext(ctx) }
func (IDsAdapter) CorrelationIDFromContext(ctx context.Context) string {
	return id.CorrelationIDFromContext(ctx)
}
func (IDsAdapter) CausationIDFromContext(ctx context.Context) string {
	return id.CausationIDFromContext(ctx)
}
func (IDsAdapter) IDsFromContext(ctx context.Context) agent.IDContextValues {
	ids := id.IDsFromContext(ctx)
	return agent.IDContextValues{
		SessionID:     ids.SessionID,
		RunID:         ids.RunID,
		ParentRunID:   ids.ParentRunID,
		LogID:         ids.LogID,
		CorrelationID: ids.CorrelationID,
		CausationID:   ids.CausationID,
	}
}
func (IDsAdapter) WithSessionID(ctx context.Context, sessionID string) context.Context {
	return id.WithSessionID(ctx, sessionID)
}
func (IDsAdapter) WithRunID(ctx context.Context, runID string) context.Context {
	return id.WithRunID(ctx, runID)
}
func (IDsAdapter) WithParentRunID(ctx context.Context, parentRunID string) context.Context {
	return id.WithParentRunID(ctx, parentRunID)
}
func (IDsAdapter) WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return id.WithCorrelationID(ctx, correlationID)
}
func (IDsAdapter) WithCausationID(ctx context.Context, causationID string) context.Context {
	return id.WithCausationID(ctx, causationID)
}
func (IDsAdapter) WithLogID(ctx context.Context, logID string) context.Context {
	return id.WithLogID(ctx, logID)
}

// LatencyAdapter forwards runtime latency logs to CLI latency output.
type LatencyAdapter struct{}

func (LatencyAdapter) PrintfWithContext(ctx context.Context, format string, args ...any) {
	clilatency.PrintfWithContext(ctx, format, args...)
}

// JSONCodecAdapter marshals runtime payloads for prompt embedding.
type JSONCodecAdapter struct{}

func (JSONCodecAdapter) Marshal(v any) ([]byte, error) {
	return jsonx.Marshal(v)
}

// GoRunnerAdapter bridges async goroutine helpers.
type GoRunnerAdapter struct{}

func (GoRunnerAdapter) Go(logger agent.Logger, name string, fn func()) {
	async.Go(logger, name, fn)
}

// WorkingDirResolverAdapter resolves working dir from request context.
type WorkingDirResolverAdapter struct{}

func (WorkingDirResolverAdapter) ResolveWorkingDir(ctx context.Context) string {
	return pathutil.GetPathResolverFromContext(ctx).ResolvePath(".")
}

// WorkspaceManagerFactoryAdapter creates workspace managers for background tasks.
type WorkspaceManagerFactoryAdapter struct{}

func (WorkspaceManagerFactoryAdapter) NewWorkspaceManager(workingDir string, logger agent.Logger) agent.WorkspaceManager {
	workingDir = strings.TrimSpace(workingDir)
	if workingDir == "" {
		return nil
	}
	return workspace.NewManager(workingDir, logger)
}
