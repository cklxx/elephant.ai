package agent

import core "alex/internal/agent/ports"

// AttachmentCarrier exposes attachments on events without coupling to concrete types.
type AttachmentCarrier interface {
	AgentEvent
	GetAttachments() map[string]core.Attachment
}
