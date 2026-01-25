package mocks

import (
	"context"
	"fmt"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/ports"
	"alex/internal/agent/ports/storage"
)

type MockContextManager struct {
	EstimateTokensFunc func(messages []ports.Message) int
	CompressFunc       func(messages []ports.Message, targetTokens int) ([]ports.Message, error)
	AutoCompactFunc    func(messages []ports.Message, limit int) ([]ports.Message, bool)
	ShouldCompressFunc func(messages []ports.Message, limit int) bool
	PreloadFunc        func(ctx context.Context) error
	BuildWindowFunc    func(ctx context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error)
	RecordTurnFunc     func(ctx context.Context, record agent.ContextTurnRecord) error
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

func (m *MockContextManager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	if m.AutoCompactFunc != nil {
		return m.AutoCompactFunc(messages, limit)
	}
	return messages, false
}

func (m *MockContextManager) ShouldCompress(messages []ports.Message, limit int) bool {
	if m.ShouldCompressFunc != nil {
		return m.ShouldCompressFunc(messages, limit)
	}
	return false
}

func (m *MockContextManager) Preload(ctx context.Context) error {
	if m.PreloadFunc != nil {
		return m.PreloadFunc(ctx)
	}
	return nil
}

func (m *MockContextManager) BuildWindow(ctx context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if m.BuildWindowFunc != nil {
		return m.BuildWindowFunc(ctx, session, cfg)
	}
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	return agent.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}

func (m *MockContextManager) RecordTurn(ctx context.Context, record agent.ContextTurnRecord) error {
	if m.RecordTurnFunc != nil {
		return m.RecordTurnFunc(ctx, record)
	}
	return nil
}
