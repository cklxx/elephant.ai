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

// Infrastructure adapter functions — wired once at startup, read by the coordinator.
// Do not reassign these at runtime; tests should inject fakes via the domain config structs.
var (
	LatencyReporter      agent.LatencyReporterFunc      = clilatency.PrintfWithContext
	JSONCodec            agent.JSONMarshalFunc           = jsonx.Marshal
	GoRunner             agent.GoRunnerFunc              = func(logger agent.Logger, name string, fn func()) { async.Go(logger, name, fn) }
	WorkingDirResolver   agent.WorkingDirResolverFunc    = func(ctx context.Context) string { return pathutil.GetPathResolverFromContext(ctx).ResolvePath(".") }
	WorkspaceManagerFactory agent.WorkspaceManagerFactoryFunc = func(workingDir string, logger agent.Logger) agent.WorkspaceManager {
		workingDir = strings.TrimSpace(workingDir)
		if workingDir == "" {
			return nil
		}
		return workspace.NewManager(workingDir, logger)
	}
)
