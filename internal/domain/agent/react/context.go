package react

import (
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/agent/textutil"
)

// snapshotSummaryFromMessages builds a short textual digest of the message
// history for context snapshots.
func snapshotSummaryFromMessages(messages []ports.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		content := textutil.NormalizeWhitespace(msg.Content)
		if content == "" {
			continue
		}
		prefix := roleSummaryPrefix(msg.Role)
		summary := prefix + content
		return textutil.TruncateWithEllipsis(summary, snapshotSummaryLimit)
	}
	return ""
}

func roleSummaryPrefix(role string) string {
	trimmed := strings.TrimSpace(role)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "assistant":
		return "Assistant: "
	case "user":
		return "User: "
	case "tool":
		return "Tool: "
	case "system":
		return ""
	default:
		if len(trimmed) == 1 {
			return strings.ToUpper(trimmed) + ": "
		}
		return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:]) + ": "
	}
}

func buildContextTurnRecord(state *agent.TaskState, messages []ports.Message, timestamp time.Time, summary string) agent.ContextTurnRecord {
	record := agent.ContextTurnRecord{
		Timestamp: timestamp,
		Summary:   summary,
		Messages:  append([]ports.Message(nil), messages...),
	}
	if state == nil {
		return record
	}
	record.SessionID = state.SessionID
	record.TurnID = state.Iterations
	record.LLMTurnSeq = state.Iterations
	record.Plans = agent.ClonePlanNodes(state.Plans)
	record.Beliefs = agent.CloneBeliefs(state.Beliefs)
	record.KnowledgeRefs = agent.CloneKnowledgeReferences(state.KnowledgeRefs)
	record.World = agent.CloneMapAny(state.WorldState)
	record.Diff = agent.CloneMapAny(state.WorldDiff)
	record.Feedback = agent.CloneFeedbackSignals(state.FeedbackSignals)
	return record
}
