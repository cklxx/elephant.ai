package llm

import (
	"strings"

	"alex/internal/domain/agent/ports"
)

func shouldEmbedAttachmentsInContent(msg ports.Message) bool {
	if len(msg.Attachments) == 0 {
		return false
	}

	if !strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
		return false
	}

	if msg.Source == ports.MessageSourceToolResult {
		return false
	}
	return true
}
