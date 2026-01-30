package okr

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleGoalContent = `---
id: q1-2026-revenue
owner: "user-uuid-001"
created: "2026-01-15"
updated: "2026-01-30"
status: active
time_window:
  start: "2026-01-01"
  end: "2026-03-31"
review_cadence: "0 9 * * 1"
notifications:
  channel: lark
  lark_chat_id: "oc_test123"
key_results:
  kr1_pipeline:
    metric: "Total enterprise pipeline value (USD)"
    baseline: 500000
    target: 1200000
    current: 780000
    progress_pct: 40.0
    confidence: high
    updated: "2026-01-29"
    source: "crm_export_jan29"
  kr2_churn:
    metric: "Monthly customer churn rate"
    baseline: 5.2
    target: 3.0
    current: 4.8
    progress_pct: 18.2
    confidence: medium
    updated: "2026-01-28"
    source: "billing_jan_report"
---

# Q1 2026: Increase recurring revenue by 30%

## Objective

Grow monthly recurring revenue from $100K to $130K by end of Q1.
`

func TestParseGoalFile(t *testing.T) {
	goal, err := ParseGoalFile([]byte(sampleGoalContent))
	if err != nil {
		t.Fatalf("ParseGoalFile: %v", err)
	}

	if goal.Meta.ID != "q1-2026-revenue" {
		t.Errorf("ID = %q, want %q", goal.Meta.ID, "q1-2026-revenue")
	}
	if goal.Meta.Owner != "user-uuid-001" {
		t.Errorf("Owner = %q, want %q", goal.Meta.Owner, "user-uuid-001")
	}
	if goal.Meta.Status != "active" {
		t.Errorf("Status = %q, want %q", goal.Meta.Status, "active")
	}
	if goal.Meta.ReviewCadence != "0 9 * * 1" {
		t.Errorf("ReviewCadence = %q, want %q", goal.Meta.ReviewCadence, "0 9 * * 1")
	}
	if goal.Meta.Notifications.Channel != "lark" {
		t.Errorf("Notifications.Channel = %q, want %q", goal.Meta.Notifications.Channel, "lark")
	}
	if goal.Meta.Notifications.LarkChatID != "oc_test123" {
		t.Errorf("Notifications.LarkChatID = %q, want %q", goal.Meta.Notifications.LarkChatID, "oc_test123")
	}
	if goal.Meta.TimeWindow.Start != "2026-01-01" {
		t.Errorf("TimeWindow.Start = %q, want %q", goal.Meta.TimeWindow.Start, "2026-01-01")
	}
	if len(goal.Meta.KeyResults) != 2 {
		t.Fatalf("KeyResults len = %d, want 2", len(goal.Meta.KeyResults))
	}

	kr1 := goal.Meta.KeyResults["kr1_pipeline"]
	if kr1.Target != 1200000 {
		t.Errorf("KR1 Target = %f, want 1200000", kr1.Target)
	}
	if kr1.Current != 780000 {
		t.Errorf("KR1 Current = %f, want 780000", kr1.Current)
	}
	if kr1.Confidence != "high" {
		t.Errorf("KR1 Confidence = %q, want %q", kr1.Confidence, "high")
	}

	if goal.Body == "" {
		t.Error("Body should not be empty")
	}
}

func TestParseGoalFile_NoFrontmatter(t *testing.T) {
	_, err := ParseGoalFile([]byte("# Just markdown"))
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestParseGoalFile_UnterminatedFrontmatter(t *testing.T) {
	_, err := ParseGoalFile([]byte("---\nid: test\n# No closing"))
	if err == nil {
		t.Fatal("expected error for unterminated frontmatter")
	}
}

func TestRenderGoalFile_Roundtrip(t *testing.T) {
	original, err := ParseGoalFile([]byte(sampleGoalContent))
	if err != nil {
		t.Fatalf("ParseGoalFile: %v", err)
	}

	rendered, err := RenderGoalFile(original)
	if err != nil {
		t.Fatalf("RenderGoalFile: %v", err)
	}

	reparsed, err := ParseGoalFile(rendered)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if reparsed.Meta.ID != original.Meta.ID {
		t.Errorf("ID mismatch: %q vs %q", reparsed.Meta.ID, original.Meta.ID)
	}
	if reparsed.Meta.Status != original.Meta.Status {
		t.Errorf("Status mismatch: %q vs %q", reparsed.Meta.Status, original.Meta.Status)
	}
	if len(reparsed.Meta.KeyResults) != len(original.Meta.KeyResults) {
		t.Errorf("KeyResults len mismatch: %d vs %d", len(reparsed.Meta.KeyResults), len(original.Meta.KeyResults))
	}
}

func TestGoalStore_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	store := NewGoalStore(OKRConfig{GoalsRoot: dir})

	goal, err := ParseGoalFile([]byte(sampleGoalContent))
	if err != nil {
		t.Fatalf("ParseGoalFile: %v", err)
	}

	if err := store.WriteGoal("q1-2026-revenue", goal); err != nil {
		t.Fatalf("WriteGoal: %v", err)
	}

	// Verify file exists
	if !store.GoalExists("q1-2026-revenue") {
		t.Fatal("goal file should exist after write")
	}

	// Read back
	read, err := store.ReadGoal("q1-2026-revenue")
	if err != nil {
		t.Fatalf("ReadGoal: %v", err)
	}
	if read.Meta.ID != "q1-2026-revenue" {
		t.Errorf("ID = %q, want %q", read.Meta.ID, "q1-2026-revenue")
	}
	if read.Meta.Status != "active" {
		t.Errorf("Status = %q, want %q", read.Meta.Status, "active")
	}
}

func TestGoalStore_ListGoals(t *testing.T) {
	dir := t.TempDir()
	store := NewGoalStore(OKRConfig{GoalsRoot: dir})

	// Empty dir
	ids, err := store.ListGoals()
	if err != nil {
		t.Fatalf("ListGoals empty: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 goals, got %d", len(ids))
	}

	// Write two goals
	goal, err := ParseGoalFile([]byte(sampleGoalContent))
	if err != nil {
		t.Fatalf("ParseGoalFile: %v", err)
	}
	if err := store.WriteGoal("goal-a", goal); err != nil {
		t.Fatalf("WriteGoal goal-a: %v", err)
	}
	goal.Meta.ID = "goal-b"
	goal.Meta.Status = "completed"
	if err := store.WriteGoal("goal-b", goal); err != nil {
		t.Fatalf("WriteGoal goal-b: %v", err)
	}

	ids, err = store.ListGoals()
	if err != nil {
		t.Fatalf("ListGoals: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 goals, got %d", len(ids))
	}
}

func TestGoalStore_ListGoals_NonexistentDir(t *testing.T) {
	store := NewGoalStore(OKRConfig{GoalsRoot: filepath.Join(t.TempDir(), "nonexistent")})
	ids, err := store.ListGoals()
	if err != nil {
		t.Fatalf("ListGoals nonexistent: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 goals, got %d", len(ids))
	}
}

func TestGoalStore_WriteGoalRaw(t *testing.T) {
	dir := t.TempDir()
	store := NewGoalStore(OKRConfig{GoalsRoot: dir})

	if err := store.WriteGoalRaw("test-goal", []byte(sampleGoalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	goal, err := store.ReadGoal("test-goal")
	if err != nil {
		t.Fatalf("ReadGoal: %v", err)
	}
	if goal.Meta.ID != "q1-2026-revenue" {
		t.Errorf("ID = %q, want %q", goal.Meta.ID, "q1-2026-revenue")
	}
}

func TestGoalStore_WriteGoalRaw_Invalid(t *testing.T) {
	dir := t.TempDir()
	store := NewGoalStore(OKRConfig{GoalsRoot: dir})

	err := store.WriteGoalRaw("bad-goal", []byte("no frontmatter here"))
	if err == nil {
		t.Fatal("expected error for invalid content")
	}
}

func TestGoalStore_DeleteGoal(t *testing.T) {
	dir := t.TempDir()
	store := NewGoalStore(OKRConfig{GoalsRoot: dir})

	if err := store.WriteGoalRaw("del-goal", []byte(sampleGoalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}
	if !store.GoalExists("del-goal") {
		t.Fatal("goal should exist")
	}

	if err := store.DeleteGoal("del-goal"); err != nil {
		t.Fatalf("DeleteGoal: %v", err)
	}
	if store.GoalExists("del-goal") {
		t.Fatal("goal should not exist after delete")
	}
}

func TestGoalStore_ListActiveGoals(t *testing.T) {
	dir := t.TempDir()
	store := NewGoalStore(OKRConfig{GoalsRoot: dir})

	// Write active goal
	if err := store.WriteGoalRaw("active-goal", []byte(sampleGoalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	// Write completed goal
	completedContent := `---
id: completed-goal
status: completed
key_results: {}
---

# Done
`
	if err := store.WriteGoalRaw("completed-goal", []byte(completedContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	active, err := store.ListActiveGoals()
	if err != nil {
		t.Fatalf("ListActiveGoals: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active goal, got %d", len(active))
	}
}

func TestGoalStore_AtomicWrite_NoPartialFile(t *testing.T) {
	dir := t.TempDir()
	store := NewGoalStore(OKRConfig{GoalsRoot: dir})

	if err := store.WriteGoalRaw("atomic-test", []byte(sampleGoalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	// Verify no .tmp file left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestGoalIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/.alex/goals/q1-2026.md", "q1-2026"},
		{"goals/test.md", "test"},
		{"file.md", "file"},
	}
	for _, tt := range tests {
		got := GoalIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("GoalIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDefaultOKRConfig(t *testing.T) {
	cfg := DefaultOKRConfig()
	if cfg.GoalsRoot == "" {
		t.Error("GoalsRoot should not be empty")
	}
}
