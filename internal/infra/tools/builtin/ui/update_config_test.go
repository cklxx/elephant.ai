package ui

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/tools/builtin/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubConfigOverrideStore implements agent.ConfigOverrideStore for testing.
type stubConfigOverrideStore struct {
	staged   *agent.ConfigOverride
	stageErr error
}

func (s *stubConfigOverrideStore) Stage(o agent.ConfigOverride) error {
	if s.stageErr != nil {
		return s.stageErr
	}
	s.staged = &o
	return nil
}

func (s *stubConfigOverrideStore) Pending() *agent.ConfigOverride { return s.staged }
func (s *stubConfigOverrideStore) PopPending() *agent.ConfigOverride {
	p := s.staged
	s.staged = nil
	return p
}
func (s *stubConfigOverrideStore) Clear() { s.staged = nil }

func ctxWithStore(store agent.ConfigOverrideStore) context.Context {
	return shared.WithConfigOverrideStore(context.Background(), store)
}

func TestUpdateConfig_SingleTemperature(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:        "call-1",
		Name:      "update_config",
		Arguments: map[string]any{"temperature": 0.1},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Content, "temperature=0.10")
	require.NotNil(t, store.staged)
	require.NotNil(t, store.staged.Temperature)
	assert.Equal(t, 0.1, *store.staged.Temperature)
}

func TestUpdateConfig_ModelSwitch(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:   "call-2",
		Name: "update_config",
		Arguments: map[string]any{
			"provider": "openai",
			"model":    "gpt-4o",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Content, "model=openai/gpt-4o")
	require.NotNil(t, store.staged)
	require.NotNil(t, store.staged.Provider)
	assert.Equal(t, "openai", *store.staged.Provider)
	require.NotNil(t, store.staged.Model)
	assert.Equal(t, "gpt-4o", *store.staged.Model)
}

func TestUpdateConfig_ProviderWithoutModel(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:        "call-3",
		Name:      "update_config",
		Arguments: map[string]any{"provider": "openai"},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Error, "should be an error result")
	assert.Contains(t, result.Content, "provider and model must be specified together")
}

func TestUpdateConfig_ModelWithoutProvider(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:        "call-4",
		Name:      "update_config",
		Arguments: map[string]any{"model": "gpt-4o"},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Content, "provider and model must be specified together")
}

func TestUpdateConfig_EmptyCall(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:        "call-5",
		Name:      "update_config",
		Arguments: map[string]any{},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Content, "at least one configuration parameter")
}

func TestUpdateConfig_NilStore(t *testing.T) {
	tool := NewUpdateConfig()

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-6",
		Name:      "update_config",
		Arguments: map[string]any{"temperature": 0.5},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Content, "config override store not available")
}

func TestUpdateConfig_AllParams(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:   "call-7",
		Name: "update_config",
		Arguments: map[string]any{
			"provider":       "claude",
			"model":          "claude-sonnet-4-20250514",
			"temperature":    0.2,
			"top_p":          0.9,
			"max_tokens":     8000,
			"max_iterations": 30,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.Content, "model=claude/claude-sonnet-4-20250514")
	assert.Contains(t, result.Content, "temperature=0.20")
	assert.Contains(t, result.Content, "top_p=0.90")
	assert.Contains(t, result.Content, "max_tokens=8000")
	assert.Contains(t, result.Content, "max_iterations=30")
}

func TestUpdateConfig_UnknownParam(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:   "call-8",
		Name: "update_config",
		Arguments: map[string]any{
			"temperature":    0.5,
			"unknown_param":  "value",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Content, "unsupported parameter: unknown_param")
}

func TestUpdateConfig_MaxIterationsOnly(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:        "call-9",
		Name:      "update_config",
		Arguments: map[string]any{"max_iterations": 100},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.Content, "max_iterations=100")
	require.NotNil(t, store.staged)
	require.NotNil(t, store.staged.MaxIterations)
	assert.Equal(t, 100, *store.staged.MaxIterations)
}

func TestUpdateConfig_InvalidTemperatureType(t *testing.T) {
	store := &stubConfigOverrideStore{}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:        "call-10",
		Name:      "update_config",
		Arguments: map[string]any{"temperature": "not_a_number"},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Content, "temperature must be a number")
}

func TestUpdateConfig_ValidationError(t *testing.T) {
	store := &stubConfigOverrideStore{
		stageErr: assert.AnError,
	}
	tool := NewUpdateConfig()

	result, err := tool.Execute(ctxWithStore(store), ports.ToolCall{
		ID:        "call-11",
		Name:      "update_config",
		Arguments: map[string]any{"temperature": 0.5},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Content, "validation failed")
}
