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

	switch msg.Source {
	case ports.MessageSourceToolResult,
		ports.MessageSourceUserHistory,
		ports.MessageSourceDebug,
		ports.MessageSourceEvaluation:
		return false
	}
	return true
}

// attachmentEmbeddingMask returns a per-message decision where at most one
// message is allowed to embed binary/image attachments. We embed only the
// latest eligible user message and keep older attachments as text references.
func attachmentEmbeddingMask(msgs []ports.Message) []bool {
	if len(msgs) == 0 {
		return nil
	}
	mask := make([]bool, len(msgs))
	latestIdx := -1
	for i := range msgs {
		if shouldEmbedAttachmentsInContent(msgs[i]) {
			latestIdx = i
		}
	}
	if latestIdx >= 0 {
		mask[latestIdx] = true
	}
	return mask
}
