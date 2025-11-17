package app

import (
	"time"

	agentports "alex/internal/agent/ports"
)

// AttachmentScanEvent is emitted when an attachment is blocked by the malware scanner.
type AttachmentScanEvent struct {
	timestamp   time.Time
	agentLevel  agentports.AgentLevel
	sessionID   string
	taskID      string
	placeholder string
	verdict     AttachmentScanVerdict
	details     string
	attachment  agentports.Attachment
}

// NewAttachmentScanEvent builds an event that can be streamed over SSE.
func NewAttachmentScanEvent(
	level agentports.AgentLevel,
	sessionID, taskID, placeholder string,
	verdict AttachmentScanVerdict,
	details string,
	attachment agentports.Attachment,
	ts time.Time,
) *AttachmentScanEvent {
	return &AttachmentScanEvent{
		timestamp:   ts,
		agentLevel:  level,
		sessionID:   sessionID,
		taskID:      taskID,
		placeholder: placeholder,
		verdict:     verdict,
		details:     details,
		attachment:  attachment,
	}
}

// EventType implements ports.AgentEvent.
func (e *AttachmentScanEvent) EventType() string { return "attachment_scan_status" }

// Timestamp implements ports.AgentEvent.
func (e *AttachmentScanEvent) Timestamp() time.Time { return e.timestamp }

// GetAgentLevel implements ports.AgentEvent.
func (e *AttachmentScanEvent) GetAgentLevel() agentports.AgentLevel { return e.agentLevel }

// GetSessionID implements ports.AgentEvent.
func (e *AttachmentScanEvent) GetSessionID() string { return e.sessionID }

// GetTaskID implements ports.AgentEvent.
func (e *AttachmentScanEvent) GetTaskID() string { return e.taskID }

// GetParentTaskID implements ports.AgentEvent.
func (e *AttachmentScanEvent) GetParentTaskID() string { return "" }

// Placeholder returns the name/placeholder of the blocked attachment.
func (e *AttachmentScanEvent) Placeholder() string { return e.placeholder }

// Verdict returns the malware scan verdict string.
func (e *AttachmentScanEvent) Verdict() AttachmentScanVerdict { return e.verdict }

// Details returns the scanner-provided details, if any.
func (e *AttachmentScanEvent) Details() string { return e.details }

// Attachment returns a copy of the original attachment metadata.
func (e *AttachmentScanEvent) Attachment() agentports.Attachment { return e.attachment }
