package mocks

import (
	"alex/internal/agent/ports"
)

type MockContextManager struct {
	EstimateTokensFunc func(messages []ports.Message) int
	CompressFunc       func(messages []ports.Message, targetTokens int) ([]ports.Message, error)
	ShouldCompressFunc func(messages []ports.Message, limit int) bool
}

func (m *MockContextManager) EstimateTokens(messages []ports.Message) int {
	if m.EstimateTokensFunc != nil {
		return m.EstimateTokensFunc(messages)
	}
	return 100
}

func (m *MockContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	if m.CompressFunc != nil {
		return m.CompressFunc(messages, targetTokens)
	}
	return messages, nil
}

func (m *MockContextManager) ShouldCompress(messages []ports.Message, limit int) bool {
	if m.ShouldCompressFunc != nil {
		return m.ShouldCompressFunc(messages, limit)
	}
	return false
}
