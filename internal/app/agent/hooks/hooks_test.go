package hooks

import (
	"context"
	"errors"
	"sync"
	"testing"

	"alex/internal/core/hook"
)

// testHook is a configurable mock hook for testing.
type testHook struct {
	name           string
	startResult    []Injection
	completeErr    error
	startCalled    int
	completeCalled int
	mu             sync.Mutex
}

func (h *testHook) Name() string { return h.name }

func (h *testHook) OnTaskStart(_ context.Context, _ TaskInfo) []Injection {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.startCalled++
	return h.startResult
}

func (h *testHook) OnTaskCompleted(_ context.Context, _ TaskResultInfo) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.completeCalled++
	return h.completeErr
}

func TestHookRuntimeRegister(t *testing.T) {
	rt := hook.NewHookRuntime()

	if len(rt.Plugins()) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(rt.Plugins()))
	}

	rt.Register(AdaptProactiveHook(&testHook{name: "a"}))
	rt.Register(AdaptProactiveHook(&testHook{name: "b"}))

	if len(rt.Plugins()) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(rt.Plugins()))
	}
}

func TestPreTaskHook_CollectsInjections(t *testing.T) {
	rt := hook.NewHookRuntime()

	rt.Register(AdaptProactiveHook(&testHook{
		name: "low-priority",
		startResult: []Injection{
			{Type: injectionSuggestion, Content: "suggestion", Source: "low-priority", Priority: 1},
		},
	}))
	rt.Register(AdaptProactiveHook(&testHook{
		name: "high-priority",
		startResult: []Injection{
			{Type: injectionSuggestion, Content: "memory", Source: "high-priority", Priority: 10},
		},
	}))

	state := &hook.TurnState{Input: "test"}

	hook.CallMany[any](context.Background(), rt, func(p hook.Plugin) (any, bool, error) {
		if h, ok := p.(hook.PreTaskHook); ok {
			return nil, false, h.PreTask(context.Background(), state)
		}
		return nil, false, nil
	})

	// Each adapter stores injections in state under "legacy_injections".
	// The last adapter to write wins — so we verify state has injections.
	raw, ok := state.Get("legacy_injections")
	if !ok {
		t.Fatal("expected legacy_injections in state")
	}
	injections, ok := raw.([]Injection)
	if !ok {
		t.Fatal("expected []Injection type")
	}
	if len(injections) == 0 {
		t.Fatal("expected non-empty injections")
	}
}

func TestPostTaskHook_AllHooksCalled(t *testing.T) {
	rt := hook.NewHookRuntime()

	h1 := &testHook{name: "a"}
	h2 := &testHook{name: "b"}
	rt.Register(AdaptProactiveHook(h1))
	rt.Register(AdaptProactiveHook(h2))

	state := &hook.TurnState{Input: "test"}
	result := &hook.TurnResult{}

	hook.CallMany[any](context.Background(), rt, func(p hook.Plugin) (any, bool, error) {
		if h, ok := p.(hook.PostTaskHook); ok {
			return nil, false, h.PostTask(context.Background(), state, result)
		}
		return nil, false, nil
	})

	if h1.completeCalled != 1 {
		t.Errorf("expected h1 completeCalled=1, got %d", h1.completeCalled)
	}
	if h2.completeCalled != 1 {
		t.Errorf("expected h2 completeCalled=1, got %d", h2.completeCalled)
	}
}

func TestPostTaskHook_ErrorDoesNotStopOthers(t *testing.T) {
	rt := hook.NewHookRuntime()

	failing := &testHook{name: "failing", completeErr: errors.New("boom")}
	succeeding := &testHook{name: "succeeding"}

	rt.Register(AdaptProactiveHook(failing))
	rt.Register(AdaptProactiveHook(succeeding))

	state := &hook.TurnState{Input: "test"}
	result := &hook.TurnResult{}

	// CallMany isolates errors — both hooks should be called.
	hook.CallMany[any](context.Background(), rt, func(p hook.Plugin) (any, bool, error) {
		if h, ok := p.(hook.PostTaskHook); ok {
			return nil, false, h.PostTask(context.Background(), state, result)
		}
		return nil, false, nil
	})

	if failing.completeCalled != 1 {
		t.Errorf("expected failing hook to be called")
	}
	if succeeding.completeCalled != 1 {
		t.Errorf("expected succeeding hook to be called despite prior error")
	}
}

func TestFormatInjectionsAsContext(t *testing.T) {
	injections := []Injection{
		{Type: injectionSuggestion, Content: "You discussed X before", Source: "memory_recall_hook", Priority: 10},
		{Type: injectionSuggestion, Content: "Consider Y", Source: "suggestion_hook", Priority: 5},
	}

	result := FormatInjectionsAsContext(injections)

	if result == "" {
		t.Fatal("expected non-empty formatted context")
	}

	// Verify structure
	expectedFragments := []string{
		"## Proactive Context",
		"### suggestion",
		"from memory_recall_hook",
		"You discussed X before",
		"### suggestion",
		"from suggestion_hook",
		"Consider Y",
	}
	for _, frag := range expectedFragments {
		if !contains(result, frag) {
			t.Errorf("expected formatted context to contain %q", frag)
		}
	}
}

func TestFormatInjectionsAsContext_Empty(t *testing.T) {
	result := FormatInjectionsAsContext(nil)
	if result != "" {
		t.Errorf("expected empty string for nil injections, got %q", result)
	}

	result = FormatInjectionsAsContext([]Injection{})
	if result != "" {
		t.Errorf("expected empty string for empty injections, got %q", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
