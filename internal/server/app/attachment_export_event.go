package app

import (
	"time"

	agentports "alex/internal/agent/ports"
)

// AttachmentExportStatus enumerates the lifecycle states emitted after the CDN
// exporter runs.
type AttachmentExportStatus string

const (
	AttachmentExportStatusSucceeded AttachmentExportStatus = "succeeded"
	AttachmentExportStatusFailed    AttachmentExportStatus = "failed"
	AttachmentExportStatusSkipped   AttachmentExportStatus = "skipped"
)

// AttachmentExportEvent is broadcast when a session's attachments are flushed
// to an external exporter.
type AttachmentExportEvent struct {
	timestamp       time.Time
	agentLevel      agentports.AgentLevel
	sessionID       string
	taskID          string
	status          AttachmentExportStatus
	attachmentCount int
	attempts        int
	duration        time.Duration
	exporterKind    string
	endpoint        string
	errorMessage    string
	updates         map[string]agentports.Attachment
}

// NewAttachmentExportEvent builds an event suitable for SSE streaming.
func NewAttachmentExportEvent(
	level agentports.AgentLevel,
	sessionID, taskID string,
	status AttachmentExportStatus,
	count, attempts int,
	duration time.Duration,
	exporterKind, endpoint, errorMessage string,
	updates map[string]agentports.Attachment,
	ts time.Time,
) *AttachmentExportEvent {
	return &AttachmentExportEvent{
		timestamp:       ts,
		agentLevel:      level,
		sessionID:       sessionID,
		taskID:          taskID,
		status:          status,
		attachmentCount: count,
		attempts:        attempts,
		duration:        duration,
		exporterKind:    exporterKind,
		endpoint:        endpoint,
		errorMessage:    errorMessage,
		updates:         cloneAttachmentPayload(updates),
	}
}

// EventType implements ports.AgentEvent.
func (e *AttachmentExportEvent) EventType() string { return "attachment_export_status" }

// Timestamp implements ports.AgentEvent.
func (e *AttachmentExportEvent) Timestamp() time.Time { return e.timestamp }

// GetAgentLevel implements ports.AgentEvent.
func (e *AttachmentExportEvent) GetAgentLevel() agentports.AgentLevel { return e.agentLevel }

// GetSessionID implements ports.AgentEvent.
func (e *AttachmentExportEvent) GetSessionID() string { return e.sessionID }

// GetTaskID implements ports.AgentEvent.
func (e *AttachmentExportEvent) GetTaskID() string { return e.taskID }

// GetParentTaskID implements ports.AgentEvent.
func (e *AttachmentExportEvent) GetParentTaskID() string { return "" }

// Status returns the export status value for serialization.
func (e *AttachmentExportEvent) Status() AttachmentExportStatus { return e.status }

// AttachmentCount returns the number of assets involved in the export.
func (e *AttachmentExportEvent) AttachmentCount() int { return e.attachmentCount }

// Attempts returns how many HTTP/webhook attempts were made.
func (e *AttachmentExportEvent) Attempts() int { return e.attempts }

// Duration returns how long the exporter spent on the request.
func (e *AttachmentExportEvent) Duration() time.Duration { return e.duration }

// ExporterKind describes the exporter implementation (http_webhook, custom, etc).
func (e *AttachmentExportEvent) ExporterKind() string { return e.exporterKind }

// Endpoint is the target location for the export if available.
func (e *AttachmentExportEvent) Endpoint() string { return e.endpoint }

// ErrorMessage includes the final error string when the export fails or is skipped.
func (e *AttachmentExportEvent) ErrorMessage() string { return e.errorMessage }

// AttachmentUpdates returns a copy of any CDN-provided attachment metadata updates.
func (e *AttachmentExportEvent) AttachmentUpdates() map[string]agentports.Attachment {
	return cloneAttachmentPayload(e.updates)
}
