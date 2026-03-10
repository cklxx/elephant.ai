package mocks

import (
	"alex/internal/domain/agent/ports"
)

type MockParser struct {
	ParseFunc func(content string) ([]ports.ToolCall, error)
}

func (m *MockParser) Parse(content string) ([]ports.ToolCall, error) {
	if m.ParseFunc != nil {
		return m.ParseFunc(content)
	}
	return []ports.ToolCall{}, nil
}
