package hooks

import (
	"context"
	"errors"
	"sync"
	"testing"
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

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry(nil)

	if r.HookCount() != 0 {
		t.Fatalf("expected 0 hooks, got %d", r.HookCount())
	}

	r.Register(&testHook{name: "a"})
	r.Register(&testHook{name: "b"})

	if r.HookCount() != 2 {
		t.Fatalf("expected 2 hooks, got %d", r.HookCount())
	}

	// nil hook should be ignored
	r.Register(nil)
	if r.HookCount() != 2 {
		t.Fatalf("expected 2 hooks after nil register, got %d", r.HookCount())
	}
}

func TestRunOnTaskStart_AggregatesAndSorts(t *testing.T) {
	r := NewRegistry(nil)

	r.Register(&testHook{
		name: "low-priority",
		startResult: []Injection{
			{Type: InjectionSuggestion, Content: "suggestion", Source: "low-priority", Priority: 1},
		},
	})
	r.Register(&testHook{
		name: "high-priority",
		startResult: []Injection{
			{Type: InjectionSuggestion, Content: "memory", Source: "high-priority", Priority: 10},
		},
	})

	injections := r.RunOnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})

	if len(injections) != 2 {
		t.Fatalf("expected 2 injections, got %d", len(injections))
	}

	// Higher priority should come first
	if injections[0].Priority != 10 {
		t.Errorf("expected first injection priority 10, got %d", injections[0].Priority)
	}
	if injections[1].Priority != 1 {
		t.Errorf("expected second injection priority 1, got %d", injections[1].Priority)
	}
}

func TestRunOnTaskStart_EmptyHooks(t *testing.T) {
	r := NewRegistry(nil)

	injections := r.RunOnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if len(injections) != 0 {
		t.Fatalf("expected 0 injections, got %d", len(injections))
	}
}

func TestRunOnTaskStart_HookReturnsNil(t *testing.T) {
	r := NewRegistry(nil)
	r.Register(&testHook{name: "nil-hook", startResult: nil})

	injections := r.RunOnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if len(injections) != 0 {
		t.Fatalf("expected 0 injections from nil-returning hook, got %d", len(injections))
	}
}

func TestRunOnTaskCompleted_AllHooksCalled(t *testing.T) {
	r := NewRegistry(nil)

	h1 := &testHook{name: "a"}
	h2 := &testHook{name: "b"}
	r.Register(h1)
	r.Register(h2)

	r.RunOnTaskCompleted(context.Background(), TaskResultInfo{TaskInput: "test"})

	if h1.completeCalled != 1 {
		t.Errorf("expected h1 completeCalled=1, got %d", h1.completeCalled)
	}
	if h2.completeCalled != 1 {
		t.Errorf("expected h2 completeCalled=1, got %d", h2.completeCalled)
	}
}

func TestRunOnTaskCompleted_ErrorDoesNotStopOthers(t *testing.T) {
	r := NewRegistry(nil)

	failing := &testHook{name: "failing", completeErr: errors.New("boom")}
	succeeding := &testHook{name: "succeeding"}

	r.Register(failing)
	r.Register(succeeding)

	r.RunOnTaskCompleted(context.Background(), TaskResultInfo{TaskInput: "test"})

	if failing.completeCalled != 1 {
		t.Errorf("expected failing hook to be called")
	}
	if succeeding.completeCalled != 1 {
		t.Errorf("expected succeeding hook to be called despite prior error")
	}
}

func TestFormatInjectionsAsContext(t *testing.T) {
	injections := []Injection{
		{Type: InjectionSuggestion, Content: "You discussed X before", Source: "memory_recall_hook", Priority: 10},
		{Type: InjectionSuggestion, Content: "Consider Y", Source: "suggestion_hook", Priority: 5},
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

func TestRunOnTaskStart_StableSortOrder(t *testing.T) {
	r := NewRegistry(nil)

	r.Register(&testHook{
		name: "first",
		startResult: []Injection{
			{Type: InjectionSuggestion, Content: "first", Source: "first", Priority: 5},
		},
	})
	r.Register(&testHook{
		name: "second",
		startResult: []Injection{
			{Type: InjectionWarning, Content: "second", Source: "second", Priority: 5},
		},
	})

	injections := r.RunOnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})

	if len(injections) != 2 {
		t.Fatalf("expected 2 injections, got %d", len(injections))
	}

	// Same priority: registration order preserved (stable sort)
	if injections[0].Source != "first" {
		t.Errorf("expected first injection from 'first', got %q", injections[0].Source)
	}
	if injections[1].Source != "second" {
		t.Errorf("expected second injection from 'second', got %q", injections[1].Source)
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
