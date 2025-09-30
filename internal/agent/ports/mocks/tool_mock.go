package mocks

import (
	"alex/internal/agent/ports"
	"context"
)

type MockToolRegistry struct {
	GetFunc      func(name string) (ports.ToolExecutor, error)
	ListFunc     func() []ports.ToolDefinition
	RegisterFunc func(tool ports.ToolExecutor) error
}

func (m *MockToolRegistry) Get(name string) (ports.ToolExecutor, error) {
	if m.GetFunc != nil {
		return m.GetFunc(name)
	}
	return &MockToolExecutor{}, nil
}

func (m *MockToolRegistry) List() []ports.ToolDefinition {
	if m.ListFunc != nil {
		return m.ListFunc()
	}
	return []ports.ToolDefinition{}
}

func (m *MockToolRegistry) Register(tool ports.ToolExecutor) error {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(tool)
	}
	return nil
}

func (m *MockToolRegistry) Unregister(name string) error {
	return nil
}

type MockToolExecutor struct {
	ExecuteFunc func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error)
}

func (m *MockToolExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, call)
	}
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Mock tool result",
	}, nil
}

func (m *MockToolExecutor) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "mock_tool"}
}

func (m *MockToolExecutor) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "mock_tool"}
}
