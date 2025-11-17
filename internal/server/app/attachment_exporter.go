package app

import (
	"context"
	"time"

	agentports "alex/internal/agent/ports"
)

// AttachmentExportResult captures exporter telemetry so downstream consumers can
// surface success/failure status and render diagnostics in the UI.
type AttachmentExportResult struct {
AttachmentCount int
Attempts        int
Duration        time.Duration
Exported        bool
Skipped         bool
Endpoint        string
ExporterKind    string
Error           error
AttachmentUpdates map[string]agentports.Attachment
}

// AttachmentExporter uploads/stages attachments outside of the sandbox so they
// remain available after the workspace is destroyed (e.g., uploading to a CDN).
//
// Implementations are invoked once per session when the final client
// disconnects from the SSE stream. They should be idempotent because repeated
// exports may occur if multiple clients attach/detach rapidly.
type AttachmentExporter interface {
	ExportSession(ctx context.Context, sessionID string, attachments map[string]agentports.Attachment) AttachmentExportResult
}

// AttachmentExporterFunc adapts ordinary functions to the AttachmentExporter
// interface.
type AttachmentExporterFunc func(ctx context.Context, sessionID string, attachments map[string]agentports.Attachment) AttachmentExportResult

// ExportSession invokes the wrapped function if it is not nil.
func (f AttachmentExporterFunc) ExportSession(ctx context.Context, sessionID string, attachments map[string]agentports.Attachment) AttachmentExportResult {
	if f == nil {
		return AttachmentExportResult{Skipped: true}
	}
	return f(ctx, sessionID, attachments)
}
