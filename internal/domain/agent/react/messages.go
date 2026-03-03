package react

import "alex/internal/domain/agent/ports"

// splitMessagesForLLM separates messages that are safe for the model from
// system-only entries (e.g., debug, evaluation) by partitioning the slice
// without deep-cloning. The caller (think()) guarantees that state.Messages
// is not mutated while the LLM call is in flight.
func splitMessagesForLLM(messages []Message) ([]Message, []Message) {
	if len(messages) == 0 {
		return nil, nil
	}
	filtered := make([]Message, 0, len(messages))
	var excluded []Message
	for _, msg := range messages {
		switch msg.Source {
		case ports.MessageSourceDebug, ports.MessageSourceEvaluation:
			excluded = append(excluded, msg)
		default:
			filtered = append(filtered, msg)
		}
	}
	return filtered, excluded
}
