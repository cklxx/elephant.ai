package mocks

import (
	"alex/internal/agent/ports"
)

type MockParser struct {
	ParseFunc    func(content string) ([]ports.ToolCall, error)
	ValidateFunc func(call ports.ToolCall, definition ports.ToolDefinition) error
}

func (m *MockParser) Parse(content string) ([]ports.ToolCall, error) {
	if m.ParseFunc != nil {
		return m.ParseFunc(content)
	}
	return []ports.ToolCall{}, nil
}

func (m *MockParser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error {
	if m.ValidateFunc != nil {
		return m.ValidateFunc(call, definition)
	}
	return nil
}
