package mocks

import (
	"context"
	"fmt"

	"alex/internal/agent/ports"
)

type MockContextManager struct {
	EstimateTokensFunc func(messages []ports.Message) int
	CompressFunc       func(messages []ports.Message, targetTokens int) ([]ports.Message, error)
	ShouldCompressFunc func(messages []ports.Message, limit int) bool
	PreloadFunc        func(ctx context.Context) error
	BuildWindowFunc    func(ctx context.Context, session *ports.Session, cfg ports.ContextWindowConfig) (ports.ContextWindow, error)
	RecordTurnFunc     func(ctx context.Context, record ports.ContextTurnRecord) error
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

func (m *MockContextManager) Preload(ctx context.Context) error {
	if m.PreloadFunc != nil {
		return m.PreloadFunc(ctx)
	}
	return nil
}

func (m *MockContextManager) BuildWindow(ctx context.Context, session *ports.Session, cfg ports.ContextWindowConfig) (ports.ContextWindow, error) {
	if m.BuildWindowFunc != nil {
		return m.BuildWindowFunc(ctx, session, cfg)
	}
	if session == nil {
		return ports.ContextWindow{}, fmt.Errorf("session required")
	}
	return ports.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}

func (m *MockContextManager) RecordTurn(ctx context.Context, record ports.ContextTurnRecord) error {
	if m.RecordTurnFunc != nil {
		return m.RecordTurnFunc(ctx, record)
	}
	return nil
}
