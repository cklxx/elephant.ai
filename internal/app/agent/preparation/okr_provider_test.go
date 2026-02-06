package preparation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/infra/tools/builtin/okr"
)

func TestNewOKRContextProvider_NoGoals(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})
	provider := NewOKRContextProvider(store)

	got := provider()
	if got == "" {
		t.Fatal("expected discovery prompt when no goals exist")
	}
	if !strings.Contains(got, "okr_write") {
		t.Error("expected discovery prompt to mention okr_write")
	}
	if !strings.Contains(got, "okr_read") {
		t.Error("expected discovery prompt to mention okr_read")
	}
	if !strings.Contains(got, "No active OKR goals") {
		t.Error("expected English discovery prompt")
	}
}

func TestNewOKRContextProvider_WithActiveGoals(t *testing.T) {
	dir := t.TempDir()
	goalContent := `---
id: Q1-2026
status: active
time_window:
  start: "2026-01-01"
  end: "2026-03-31"
key_results:
  revenue:
    current: 50
    target: 100
    progress_pct: 50
    confidence: "medium"
review_cadence: weekly
---

## Strategy
Focus on growth.
`
	if err := os.WriteFile(filepath.Join(dir, "Q1-2026.md"), []byte(goalContent), 0o644); err != nil {
		t.Fatalf("failed to write test goal: %v", err)
	}

	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})
	provider := NewOKRContextProvider(store)

	got := provider()
	if got == "" {
		t.Fatal("expected goals summary")
	}
	if !strings.Contains(got, "Active OKR Goals") {
		t.Error("expected summary to contain 'Active OKR Goals'")
	}
	if !strings.Contains(got, "Q1-2026") {
		t.Error("expected summary to contain goal ID")
	}
	if !strings.Contains(got, "revenue") {
		t.Error("expected summary to contain key result ID")
	}
}

func TestNewOKRContextProvider_InactiveGoals(t *testing.T) {
	dir := t.TempDir()
	goalContent := `---
id: old-goal
status: completed
---

Done.
`
	if err := os.WriteFile(filepath.Join(dir, "old-goal.md"), []byte(goalContent), 0o644); err != nil {
		t.Fatalf("failed to write test goal: %v", err)
	}

	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})
	provider := NewOKRContextProvider(store)

	got := provider()
	if strings.Contains(got, "Active OKR Goals") {
		t.Error("completed goals should not appear as active")
	}
	if !strings.Contains(got, "No active OKR goals") {
		t.Error("expected English discovery prompt when no active goals")
	}
}

func TestNewOKRContextProvider_NonexistentDir(t *testing.T) {
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: "/nonexistent/path/to/goals"})
	provider := NewOKRContextProvider(store)

	got := provider()
	// ListActiveGoals returns nil, nil for nonexistent dir → discovery prompt
	if !strings.Contains(got, "No active OKR goals") {
		t.Error("expected English discovery prompt for nonexistent dir")
	}
}
