package domain

import (
	"strings"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/ports"
)

// snapshotSummaryFromMessages builds a short textual digest of the message
// history for context snapshots.
func snapshotSummaryFromMessages(messages []ports.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		content := normalizeWhitespace(msg.Content)
		if content == "" {
			continue
		}
		prefix := roleSummaryPrefix(msg.Role)
		summary := prefix + content
		return truncateWithEllipsis(summary, snapshotSummaryLimit)
	}
	return ""
}

func normalizeWhitespace(input string) string {
	fields := strings.Fields(input)
	return strings.Join(fields, " ")
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

func truncateWithEllipsis(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	if limit == 1 {
		return "…"
	}
	trimmed := strings.TrimSpace(string(runes[:limit-1]))
	if trimmed == "" {
		trimmed = string(runes[:limit-1])
	}
	return trimmed + "…"
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
	record.Plans = clonePlanNodes(state.Plans)
	record.Beliefs = cloneBeliefs(state.Beliefs)
	record.KnowledgeRefs = cloneKnowledgeReferences(state.KnowledgeRefs)
	record.World = cloneMapAny(state.WorldState)
	record.Diff = cloneMapAny(state.WorldDiff)
	record.Feedback = cloneFeedbackSignals(state.FeedbackSignals)
	return record
}

func clonePlanNodes(nodes []agent.PlanNode) []agent.PlanNode {
	if len(nodes) == 0 {
		return nil
	}
	cloned := make([]agent.PlanNode, 0, len(nodes))
	for _, node := range nodes {
		copyNode := agent.PlanNode{
			ID:          node.ID,
			Title:       node.Title,
			Status:      node.Status,
			Description: node.Description,
		}
		copyNode.Children = clonePlanNodes(node.Children)
		cloned = append(cloned, copyNode)
	}
	return cloned
}

func cloneBeliefs(beliefs []agent.Belief) []agent.Belief {
	if len(beliefs) == 0 {
		return nil
	}
	cloned := make([]agent.Belief, 0, len(beliefs))
	for _, belief := range beliefs {
		cloned = append(cloned, agent.Belief{
			Statement:  belief.Statement,
			Confidence: belief.Confidence,
			Source:     belief.Source,
		})
	}
	return cloned
}

func cloneKnowledgeReferences(refs []agent.KnowledgeReference) []agent.KnowledgeReference {
	if len(refs) == 0 {
		return nil
	}
	cloned := make([]agent.KnowledgeReference, 0, len(refs))
	for _, ref := range refs {
		copyRef := agent.KnowledgeReference{
			ID:          ref.ID,
			Description: ref.Description,
		}
		copyRef.SOPRefs = append([]string(nil), ref.SOPRefs...)
		copyRef.RAGCollections = append([]string(nil), ref.RAGCollections...)
		copyRef.MemoryKeys = append([]string(nil), ref.MemoryKeys...)
		cloned = append(cloned, copyRef)
	}
	return cloned
}

func cloneFeedbackSignals(signals []agent.FeedbackSignal) []agent.FeedbackSignal {
	if len(signals) == 0 {
		return nil
	}
	cloned := make([]agent.FeedbackSignal, len(signals))
	copy(cloned, signals)
	return cloned
}

func cloneMapAny(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = cloneWorldValue(value)
	}
	return cloned
}

func cloneWorldValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMapAny(v)
	case []map[string]any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]map[string]any, len(v))
		for i := range v {
			cloned[i] = cloneMapAny(v[i])
		}
		return cloned
	case []string:
		return append([]string(nil), v...)
	case []any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]any, len(v))
		for i := range v {
			cloned[i] = cloneWorldValue(v[i])
		}
		return cloned
	default:
		return v
	}
}
