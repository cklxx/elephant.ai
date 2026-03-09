package react

import (
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/textutil"
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

var rolePrefixMap = map[string]string{
	"assistant": "Assistant: ",
	"user":      "User: ",
	"tool":      "Tool: ",
	"system":    "",
}

func roleSummaryPrefix(role string) string {
	trimmed := strings.TrimSpace(role)
	if trimmed == "" {
		return ""
	}
	if prefix, ok := rolePrefixMap[strings.ToLower(trimmed)]; ok {
		return prefix
	}
	return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:]) + ": "
}

func buildContextTurnRecord(state *agent.TaskState, messages []ports.Message, timestamp time.Time, summary string) agent.ContextTurnRecord {
	record := agent.ContextTurnRecord{
		Timestamp:    timestamp,
		Summary:      summary,
		MessageCount: len(messages),
	}
	if state == nil {
		return record
	}
	record.SessionID = state.SessionID
	record.TurnID = state.Iterations
	record.LLMTurnSeq = state.Iterations
	record.Plans = agent.ClonePlanNodes(state.Plans)
	cog := state.CognitiveOrEmpty()
	record.Beliefs = agent.CloneBeliefs(cog.Beliefs)
	record.KnowledgeRefs = agent.CloneKnowledgeReferences(cog.KnowledgeRefs)
	record.World = agent.CloneMapAny(cog.WorldState)
	record.Diff = agent.CloneMapAny(cog.WorldDiff)
	record.Feedback = agent.CloneFeedbackSignals(cog.FeedbackSignals)
	record.Messages = agent.CloneMessages(messages)
	return record
}
