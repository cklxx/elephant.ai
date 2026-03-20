package hook

import (
	"context"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// test plugin
// ---------------------------------------------------------------------------

type testPlugin struct {
	name     string
	priority int
}

func (p testPlugin) Name() string   { return p.name }
func (p testPlugin) Priority() int  { return p.priority }

// ---------------------------------------------------------------------------
// TurnState tests
// ---------------------------------------------------------------------------

func TestTurnState_SetGet_NilMap(t *testing.T) {
	s := &TurnState{} // Values is nil
	s.Set("key", "val")
	v, ok := s.Get("key")
	if !ok || v != "val" {
		t.Fatalf("Get after Set on nil map: got %v, %v", v, ok)
	}
}

func TestTurnState_Get_MissingKey(t *testing.T) {
	s := &TurnState{}
	v, ok := s.Get("missing")
	if ok || v != nil {
		t.Fatalf("expected (nil, false), got (%v, %v)", v, ok)
	}
}

func TestTurnState_GetString(t *testing.T) {
	s := &TurnState{}
	s.Set("name", "alice")
	s.Set("count", 42)

	if got := s.GetString("name"); got != "alice" {
		t.Fatalf("GetString(name) = %q, want %q", got, "alice")
	}
	if got := s.GetString("count"); got != "" {
		t.Fatalf("GetString(count) should return empty for non-string, got %q", got)
	}
	if got := s.GetString("missing"); got != "" {
		t.Fatalf("GetString(missing) should return empty, got %q", got)
	}
}

func TestTurnState_GetString_NilMap(t *testing.T) {
	s := &TurnState{}
	if got := s.GetString("any"); got != "" {
		t.Fatalf("GetString on nil map should return empty, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// HookRuntime.Register — priority ordering
// ---------------------------------------------------------------------------

func TestRegister_PriorityOrder(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "low", priority: 1})
	r.Register(testPlugin{name: "high", priority: 100})
	r.Register(testPlugin{name: "mid", priority: 50})

	ps := r.Plugins()
	if len(ps) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(ps))
	}
	want := []string{"high", "mid", "low"}
	for i, w := range want {
		if ps[i].Name() != w {
			t.Fatalf("plugins[%d].Name() = %q, want %q", i, ps[i].Name(), w)
		}
	}
}

func TestRegister_StableOrder(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "a", priority: 10})
	r.Register(testPlugin{name: "b", priority: 10})
	r.Register(testPlugin{name: "c", priority: 10})

	ps := r.Plugins()
	// SliceStable preserves insertion order for equal priorities.
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if ps[i].Name() != w {
			t.Fatalf("plugins[%d].Name() = %q, want %q (stable order broken)", i, ps[i].Name(), w)
		}
	}
}

// ---------------------------------------------------------------------------
// Plugins returns defensive copy
// ---------------------------------------------------------------------------

func TestPlugins_DefensiveCopy(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "a", priority: 1})

	snap := r.Plugins()
	snap[0] = testPlugin{name: "mutated", priority: 999}

	ps := r.Plugins()
	if ps[0].Name() != "a" {
		t.Fatal("Plugins() did not return a defensive copy")
	}
}

// ---------------------------------------------------------------------------
// CallFirst
// ---------------------------------------------------------------------------

func TestCallFirst_ReturnsFirstHandled(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "high", priority: 100})
	r.Register(testPlugin{name: "low", priority: 1})

	result, err := CallFirst[string](context.Background(), r, func(p Plugin) (string, bool, error) {
		if p.Name() == "high" {
			return "high-result", true, nil
		}
		return "", false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "high-result" {
		t.Fatalf("got %q, want %q", result, "high-result")
	}
}

func TestCallFirst_SkipsUnhandled(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "skip", priority: 100})
	r.Register(testPlugin{name: "handle", priority: 1})

	result, err := CallFirst[string](context.Background(), r, func(p Plugin) (string, bool, error) {
		if p.Name() == "handle" {
			return "found", true, nil
		}
		return "", false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "found" {
		t.Fatalf("got %q, want %q", result, "found")
	}
}

func TestCallFirst_ZeroValueWhenNoneHandles(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "a", priority: 1})

	result, err := CallFirst[string](context.Background(), r, func(_ Plugin) (string, bool, error) {
		return "", false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("expected zero value, got %q", result)
	}
}

func TestCallFirst_ZeroValueNoPlugins(t *testing.T) {
	r := NewHookRuntime()
	result, err := CallFirst[int](context.Background(), r, func(_ Plugin) (int, bool, error) {
		return 42, true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != 0 {
		t.Fatalf("expected 0, got %d", result)
	}
}

func TestCallFirst_StopsOnError(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "err", priority: 100})
	r.Register(testPlugin{name: "ok", priority: 1})

	called := false
	_, err := CallFirst[string](context.Background(), r, func(p Plugin) (string, bool, error) {
		if p.Name() == "err" {
			return "", false, errors.New("boom")
		}
		called = true
		return "ok", true, nil
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
	if called {
		t.Fatal("should not have called lower-priority plugin after error")
	}
}

func TestCallFirst_RespectsContextCancellation(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "a", priority: 1})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CallFirst[string](ctx, r, func(_ Plugin) (string, bool, error) {
		t.Fatal("should not be called on cancelled context")
		return "", false, nil
	})
	if err == nil {
		t.Fatal("expected context error")
	}
}

// ---------------------------------------------------------------------------
// CallMany
// ---------------------------------------------------------------------------

func TestCallMany_CollectsAllHandled(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "a", priority: 100})
	r.Register(testPlugin{name: "b", priority: 50})
	r.Register(testPlugin{name: "c", priority: 1})

	results, errs := CallMany[string](context.Background(), r, func(p Plugin) (string, bool, error) {
		return p.Name() + "-result", true, nil
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	want := []string{"a-result", "b-result", "c-result"}
	for i, w := range want {
		if results[i] != w {
			t.Fatalf("results[%d] = %q, want %q", i, results[i], w)
		}
	}
}

func TestCallMany_ContinuesAfterError(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "ok1", priority: 100})
	r.Register(testPlugin{name: "fail", priority: 50})
	r.Register(testPlugin{name: "ok2", priority: 1})

	results, errs := CallMany[string](context.Background(), r, func(p Plugin) (string, bool, error) {
		if p.Name() == "fail" {
			return "", false, errors.New("fail-error")
		}
		return p.Name(), true, nil
	})
	if len(errs) != 1 || errs[0].Error() != "fail-error" {
		t.Fatalf("expected 1 error 'fail-error', got %v", errs)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != "ok1" || results[1] != "ok2" {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestCallMany_SkipsUnhandled(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "a", priority: 10})
	r.Register(testPlugin{name: "b", priority: 5})

	results, errs := CallMany[string](context.Background(), r, func(p Plugin) (string, bool, error) {
		if p.Name() == "a" {
			return "yes", true, nil
		}
		return "", false, nil // unhandled
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(results) != 1 || results[0] != "yes" {
		t.Fatalf("expected [yes], got %v", results)
	}
}

func TestCallMany_RespectsContextCancellation(t *testing.T) {
	r := NewHookRuntime()
	r.Register(testPlugin{name: "a", priority: 1})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, errs := CallMany[string](ctx, r, func(_ Plugin) (string, bool, error) {
		t.Fatal("should not be called on cancelled context")
		return "", false, nil
	})
	if len(results) != 0 {
		t.Fatalf("expected no results, got %v", results)
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 context error, got %v", errs)
	}
}
