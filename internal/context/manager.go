package context

import (
	"alex/internal/agent/ports"
)

type manager struct {
	threshold float64
}

func NewManager() ports.ContextManager {
	return &manager{threshold: 0.8}
}

func (m *manager) EstimateTokens(messages []ports.Message) int {
	count := 0
	for _, msg := range messages {
		count += len(msg.Content) / 4 // Rough estimate: 1 token ≈ 4 chars
	}
	return count
}

func (m *manager) ShouldCompress(messages []ports.Message, limit int) bool {
	tokenCount := m.EstimateTokens(messages)
	return float64(tokenCount) > float64(limit)*m.threshold
}

func (m *manager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	currentTokens := m.EstimateTokens(messages)
	if currentTokens <= targetTokens {
		return messages, nil
	}

	// Simple strategy: keep first (system) and last 10 messages
	if len(messages) <= 11 {
		return messages, nil
	}

	result := []ports.Message{messages[0]} // Keep system
	recent := messages[len(messages)-10:]  // Keep last 10
	result = append(result, ports.Message{
		Role:    "system",
		Content: "[Previous conversation compressed...]",
	})
	result = append(result, recent...)
	return result, nil
}
