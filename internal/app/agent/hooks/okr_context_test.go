package hooks

import (
	"context"
	"strings"
	"testing"

	"alex/internal/logging"
	"alex/internal/tools/builtin/okr"
)

const testGoalContent = `---
id: test-goal
owner: "user-001"
created: "2026-01-15"
updated: "2026-01-30"
status: active
time_window:
  start: "2026-01-01"
  end: "2026-03-31"
review_cadence: "0 9 * * 1"
key_results:
  kr1:
    metric: "Revenue"
    baseline: 100
    target: 200
    current: 150
    progress_pct: 50.0
    confidence: high
    updated: "2026-01-29"
    source: "dashboard"
---

# Test Goal
`

func TestOKRContextHook_Name(t *testing.T) {
	hook := NewOKRContextHook(nil, nil, OKRContextConfig{})
	if hook.Name() != "okr_context" {
		t.Errorf("Name() = %q, want okr_context", hook.Name())
	}
}

func TestOKRContextHook_Disabled(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})
	hook := NewOKRContextHook(store, logging.NewComponentLogger("test"), OKRContextConfig{
		Enabled:    false,
		AutoInject: true,
	})

	injections := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if len(injections) != 0 {
		t.Errorf("expected 0 injections when disabled, got %d", len(injections))
	}
}

func TestOKRContextHook_NoAutoInject(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})
	hook := NewOKRContextHook(store, logging.NewComponentLogger("test"), OKRContextConfig{
		Enabled:    true,
		AutoInject: false,
	})

	injections := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if len(injections) != 0 {
		t.Errorf("expected 0 injections when auto_inject=false, got %d", len(injections))
	}
}

func TestOKRContextHook_NoActiveGoals(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})
	hook := NewOKRContextHook(store, logging.NewComponentLogger("test"), OKRContextConfig{
		Enabled:    true,
		AutoInject: true,
	})

	injections := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if len(injections) != 0 {
		t.Errorf("expected 0 injections with no goals, got %d", len(injections))
	}
}

func TestOKRContextHook_InjectsActiveGoals(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})
	if err := store.WriteGoalRaw("test-goal", []byte(testGoalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	hook := NewOKRContextHook(store, logging.NewComponentLogger("test"), OKRContextConfig{
		Enabled:    true,
		AutoInject: true,
	})

	injections := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: "review my goals"})
	if len(injections) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(injections))
	}

	inj := injections[0]
	if inj.Type != InjectionOKRContext {
		t.Errorf("Type = %q, want %q", inj.Type, InjectionOKRContext)
	}
	if inj.Source != "okr_context" {
		t.Errorf("Source = %q, want okr_context", inj.Source)
	}
	if inj.Priority != 80 {
		t.Errorf("Priority = %d, want 80", inj.Priority)
	}
	if !strings.Contains(inj.Content, "test-goal") {
		t.Errorf("expected goal ID in content, got %q", inj.Content)
	}
	if !strings.Contains(inj.Content, "50.0%") {
		t.Errorf("expected progress in content, got %q", inj.Content)
	}
	if !strings.Contains(inj.Content, "0 9 * * 1") {
		t.Errorf("expected cadence in content, got %q", inj.Content)
	}
}

func TestOKRContextHook_SkipsCompletedGoals(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})

	completedContent := `---
id: done-goal
status: completed
key_results: {}
---

# Done
`
	if err := store.WriteGoalRaw("done-goal", []byte(completedContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	hook := NewOKRContextHook(store, logging.NewComponentLogger("test"), OKRContextConfig{
		Enabled:    true,
		AutoInject: true,
	})

	injections := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if len(injections) != 0 {
		t.Errorf("expected 0 injections for completed goal, got %d", len(injections))
	}
}

func TestOKRContextHook_OnTaskCompleted_Noop(t *testing.T) {
	hook := NewOKRContextHook(nil, nil, OKRContextConfig{})
	err := hook.OnTaskCompleted(context.Background(), TaskResultInfo{})
	if err != nil {
		t.Errorf("OnTaskCompleted: unexpected error: %v", err)
	}
}
