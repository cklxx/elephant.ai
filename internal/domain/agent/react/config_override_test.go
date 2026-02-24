package react

import (
	"sync"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	"alex/internal/domain/agent/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// --- Store lifecycle tests ---

func TestConfigOverrideStore_InitiallyNil(t *testing.T) {
	store := newConfigOverrideStore()
	assert.Nil(t, store.Pending())
}

func TestConfigOverrideStore_StagePending(t *testing.T) {
	store := newConfigOverrideStore()
	err := store.Stage(agent.ConfigOverride{Temperature: ptr(0.5)})
	require.NoError(t, err)

	p := store.Pending()
	require.NotNil(t, p)
	require.NotNil(t, p.Temperature)
	assert.Equal(t, 0.5, *p.Temperature)
}

func TestConfigOverrideStore_Clear(t *testing.T) {
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{Temperature: ptr(0.3)})
	store.Clear()
	assert.Nil(t, store.Pending())
}

func TestConfigOverrideStore_MergeOverrides(t *testing.T) {
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{Temperature: ptr(0.5)})
	_ = store.Stage(agent.ConfigOverride{MaxTokens: ptr(4096)})

	p := store.Pending()
	require.NotNil(t, p)
	require.NotNil(t, p.Temperature)
	assert.Equal(t, 0.5, *p.Temperature)
	require.NotNil(t, p.MaxTokens)
	assert.Equal(t, 4096, *p.MaxTokens)
}

func TestConfigOverrideStore_OverwriteSameField(t *testing.T) {
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{Temperature: ptr(0.5)})
	_ = store.Stage(agent.ConfigOverride{Temperature: ptr(0.9)})

	p := store.Pending()
	require.NotNil(t, p)
	assert.Equal(t, 0.9, *p.Temperature)
}

func TestConfigOverrideStore_PendingReturnsCopy(t *testing.T) {
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{Temperature: ptr(0.5)})

	p1 := store.Pending()
	p2 := store.Pending()
	*p1.Temperature = 99.0
	assert.Equal(t, 0.5, *p2.Temperature)
}

// --- Validation tests ---

func TestValidateConfigOverride_ValidRanges(t *testing.T) {
	cases := []struct {
		name     string
		override agent.ConfigOverride
	}{
		{"zero temperature", agent.ConfigOverride{Temperature: ptr(0.0)}},
		{"max temperature", agent.ConfigOverride{Temperature: ptr(2.0)}},
		{"mid temperature", agent.ConfigOverride{Temperature: ptr(1.0)}},
		{"min top_p", agent.ConfigOverride{TopP: ptr(0.0)}},
		{"max top_p", agent.ConfigOverride{TopP: ptr(1.0)}},
		{"min max_tokens", agent.ConfigOverride{MaxTokens: ptr(1)}},
		{"max max_tokens", agent.ConfigOverride{MaxTokens: ptr(128000)}},
		{"min max_iterations", agent.ConfigOverride{MaxIterations: ptr(1)}},
		{"max max_iterations", agent.ConfigOverride{MaxIterations: ptr(200)}},
		{"empty override", agent.ConfigOverride{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, validateConfigOverride(tc.override))
		})
	}
}

func TestValidateConfigOverride_InvalidRanges(t *testing.T) {
	cases := []struct {
		name     string
		override agent.ConfigOverride
		errMsg   string
	}{
		{"temperature too low", agent.ConfigOverride{Temperature: ptr(-0.1)}, "temperature"},
		{"temperature too high", agent.ConfigOverride{Temperature: ptr(2.1)}, "temperature"},
		{"top_p too low", agent.ConfigOverride{TopP: ptr(-0.1)}, "top_p"},
		{"top_p too high", agent.ConfigOverride{TopP: ptr(1.1)}, "top_p"},
		{"max_tokens zero", agent.ConfigOverride{MaxTokens: ptr(0)}, "max_tokens"},
		{"max_tokens negative", agent.ConfigOverride{MaxTokens: ptr(-1)}, "max_tokens"},
		{"max_tokens too high", agent.ConfigOverride{MaxTokens: ptr(128001)}, "max_tokens"},
		{"max_iterations zero", agent.ConfigOverride{MaxIterations: ptr(0)}, "max_iterations"},
		{"max_iterations too high", agent.ConfigOverride{MaxIterations: ptr(201)}, "max_iterations"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigOverride(tc.override)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestConfigOverrideStore_StageRejectsInvalid(t *testing.T) {
	store := newConfigOverrideStore()
	err := store.Stage(agent.ConfigOverride{Temperature: ptr(5.0)})
	require.Error(t, err)
	assert.Nil(t, store.Pending())
}

// --- Concurrency test ---

func TestConfigOverrideStore_ConcurrentAccess(t *testing.T) {
	store := newConfigOverrideStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			temp := float64(i) / 50.0
			_ = store.Stage(agent.ConfigOverride{Temperature: &temp})
			_ = store.Pending()
		}(i)
	}
	wg.Wait()

	p := store.Pending()
	require.NotNil(t, p)
	require.NotNil(t, p.Temperature)
}

// --- applyPendingConfigOverrides tests ---

func TestApplyPendingConfigOverrides_Temperature(t *testing.T) {
	engine := newReactEngineForTest(10)
	engine.completion.temperature = 0.7
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{Temperature: ptr(0.1)})

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, 0.1, engine.completion.temperature)
	assert.Nil(t, store.Pending(), "pending should be cleared after apply")
}

func TestApplyPendingConfigOverrides_MaxIterations(t *testing.T) {
	engine := newReactEngineForTest(10)
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{MaxIterations: ptr(50)})

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, 50, engine.maxIterations)
}

func TestApplyPendingConfigOverrides_TopP(t *testing.T) {
	engine := newReactEngineForTest(10)
	engine.completion.topP = 1.0
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{TopP: ptr(0.9)})

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, 0.9, engine.completion.topP)
}

func TestApplyPendingConfigOverrides_MaxTokens(t *testing.T) {
	engine := newReactEngineForTest(10)
	engine.completion.maxTokens = 12000
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{MaxTokens: ptr(4096)})

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, 4096, engine.completion.maxTokens)
}

func TestApplyPendingConfigOverrides_StopSequences(t *testing.T) {
	engine := newReactEngineForTest(10)
	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{StopSequences: []string{"STOP", "END"}})

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, []string{"STOP", "END"}, engine.completion.stopSequences)
}

func TestApplyPendingConfigOverrides_NilPendingNoOp(t *testing.T) {
	engine := newReactEngineForTest(10)
	engine.completion.temperature = 0.7
	store := newConfigOverrideStore()

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, 0.7, engine.completion.temperature)
}

func TestApplyPendingConfigOverrides_NilStoreNoOp(t *testing.T) {
	engine := newReactEngineForTest(10)
	r := &reactRuntime{engine: engine}
	r.applyPendingConfigOverrides()
}

func TestApplyPendingConfigOverrides_ModelSwitch(t *testing.T) {
	var capturedProvider, capturedModel string
	newMock := &mocks.MockLLMClient{
		ModelFunc: func() string { return "new-model" },
	}

	engine := newReactEngineForTest(10)
	engine.llmRebuilder = func(provider, model string) (llm.StreamingLLMClient, error) {
		capturedProvider = provider
		capturedModel = model
		return newMock, nil
	}

	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{
		Provider: ptr("openai"),
		Model:    ptr("gpt-4o"),
	})

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, "openai", capturedProvider)
	assert.Equal(t, "gpt-4o", capturedModel)
	assert.Equal(t, newMock, r.services.LLM, "services.LLM should be replaced")
}

func TestApplyPendingConfigOverrides_ModelSwitchRequiresBothFields(t *testing.T) {
	engine := newReactEngineForTest(10)
	rebuilderCalled := false
	engine.llmRebuilder = func(provider, model string) (llm.StreamingLLMClient, error) {
		rebuilderCalled = true
		return nil, nil
	}

	store := newConfigOverrideStore()
	// Only provider, no model — rebuilder should NOT be called.
	_ = store.Stage(agent.ConfigOverride{Provider: ptr("openai")})

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: &mocks.MockLLMClient{}},
	}
	r.applyPendingConfigOverrides()

	assert.False(t, rebuilderCalled, "rebuilder should not be called without both provider and model")
}

func TestApplyPendingConfigOverrides_NilRebuilderSkipsModelSwitch(t *testing.T) {
	engine := newReactEngineForTest(10)
	// llmRebuilder is nil by default.

	store := newConfigOverrideStore()
	_ = store.Stage(agent.ConfigOverride{
		Provider: ptr("openai"),
		Model:    ptr("gpt-4o"),
	})
	originalClient := &mocks.MockLLMClient{}

	r := &reactRuntime{
		engine:          engine,
		configOverrides: store,
		services:        Services{LLM: originalClient},
	}
	r.applyPendingConfigOverrides()

	assert.Equal(t, originalClient, r.services.LLM, "LLM client should be unchanged when rebuilder is nil")
}
