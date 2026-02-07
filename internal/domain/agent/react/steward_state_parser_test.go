package react

import (
	"fmt"
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestExtractNewState_NoTag(t *testing.T) {
	content := "Just a normal response."
	state, cleaned, err := ExtractNewState(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state when no tag present")
	}
	if cleaned != content {
		t.Errorf("cleaned = %q, want %q", cleaned, content)
	}
}

func TestExtractNewState_UnclosedTag(t *testing.T) {
	content := "Here is <NEW_STATE>{\"version\":1} with no close"
	state, cleaned, err := ExtractNewState(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for unclosed tag")
	}
	if cleaned != content {
		t.Errorf("cleaned should be unchanged for unclosed tag")
	}
}

func TestExtractNewState_EmptyBlock(t *testing.T) {
	content := "Before <NEW_STATE>  </NEW_STATE> after"
	state, cleaned, err := ExtractNewState(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for empty block")
	}
	// Block should be removed.
	if strings.Contains(cleaned, "NEW_STATE") {
		t.Errorf("cleaned should not contain NEW_STATE tags: %q", cleaned)
	}
}

func TestExtractNewState_ValidJSON(t *testing.T) {
	json := `{"version":2,"goal":"Ship it","context":"Phase 2","plan":[],"task_graph":[],"decisions":[],"artifacts":[],"risks":[],"evidence_index":[]}`
	content := "Before text\n<NEW_STATE>\n" + json + "\n</NEW_STATE>\nAfter text"

	state, cleaned, err := ExtractNewState(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.Version != 2 {
		t.Errorf("Version = %d, want 2", state.Version)
	}
	if state.Goal != "Ship it" {
		t.Errorf("Goal = %q, want 'Ship it'", state.Goal)
	}
	if strings.Contains(cleaned, "NEW_STATE") {
		t.Errorf("cleaned should not contain NEW_STATE tags: %q", cleaned)
	}
	if !strings.Contains(cleaned, "Before text") {
		t.Errorf("cleaned should preserve surrounding text")
	}
	if !strings.Contains(cleaned, "After text") {
		t.Errorf("cleaned should preserve surrounding text")
	}
}

func TestExtractNewState_InvalidJSON(t *testing.T) {
	content := "Text <NEW_STATE>not valid json</NEW_STATE> more"
	state, cleaned, err := ExtractNewState(content)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if state != nil {
		t.Error("expected nil state for invalid JSON")
	}
	if strings.Contains(cleaned, "NEW_STATE") {
		t.Errorf("cleaned should not contain NEW_STATE tags even on error: %q", cleaned)
	}
}

func TestExtractNewState_Oversize(t *testing.T) {
	// Build a state with a very long goal that exceeds MaxStewardStateChars.
	longGoal := strings.Repeat("目标", agent.MaxStewardStateChars)
	json := `{"version":1,"goal":"` + longGoal + `"}`
	content := "<NEW_STATE>" + json + "</NEW_STATE>"

	state, cleaned, err := ExtractNewState(content)
	if err == nil {
		t.Error("expected oversize error")
	}
	if !IsStewardStateOversize(err) {
		t.Errorf("expected stewardStateOversizeError, got %T: %v", err, err)
	}
	if state == nil {
		t.Fatal("expected parsed state for oversize content")
	}
	if strings.Contains(cleaned, "NEW_STATE") {
		t.Errorf("cleaned should not contain NEW_STATE tags: %q", cleaned)
	}
}

func TestValidateStewardEvidenceRefs(t *testing.T) {
	valid := &agent.StewardState{
		Decisions: []agent.Decision{
			{Conclusion: "Choose path A", EvidenceRef: "ref-1"},
		},
		EvidenceIndex: []agent.EvidenceRef{
			{Ref: "ref-1", Source: "tool://web_search", Summary: "source"},
		},
	}
	if issues := ValidateStewardEvidenceRefs(valid); len(issues) != 0 {
		t.Fatalf("expected no validation issues, got %v", issues)
	}

	invalid := &agent.StewardState{
		Decisions: []agent.Decision{
			{Conclusion: "No ref"},
			{Conclusion: "Bad ref", EvidenceRef: "missing"},
		},
		EvidenceIndex: []agent.EvidenceRef{
			{Ref: "ref-ok", Source: "tool://x", Summary: "x"},
		},
	}
	issues := ValidateStewardEvidenceRefs(invalid)
	if len(issues) != 2 {
		t.Fatalf("expected 2 validation issues, got %d: %v", len(issues), issues)
	}
}

func TestCompressStewardStateForLimit(t *testing.T) {
	state := &agent.StewardState{
		Version: 1,
		Goal:    strings.Repeat("目标", 220),
		Context: strings.Repeat("上下文", 260),
		Plan: []agent.StewardAction{
			{Input: "a", Tool: "t1"},
			{Input: "b", Tool: "t2"},
			{Input: "c", Tool: "t3"},
			{Input: "d", Tool: "t4"},
		},
		TaskGraph: []agent.TaskGraphNode{
			{ID: "1", Title: "done-1", Status: "done"},
			{ID: "2", Title: "done-2", Status: "done"},
			{ID: "3", Title: "in-progress", Status: "in_progress"},
			{ID: "4", Title: "pending-1", Status: "pending"},
			{ID: "5", Title: "pending-2", Status: "pending"},
			{ID: "6", Title: "pending-3", Status: "pending"},
			{ID: "7", Title: "pending-4", Status: "pending"},
		},
		Decisions: []agent.Decision{
			{Conclusion: "d1", EvidenceRef: "r1", Alternatives: strings.Repeat("alt", 120)},
			{Conclusion: "d2", EvidenceRef: "r2", Alternatives: strings.Repeat("alt", 120)},
			{Conclusion: "d3", EvidenceRef: "r3", Alternatives: strings.Repeat("alt", 120)},
			{Conclusion: "d4", EvidenceRef: "r4", Alternatives: strings.Repeat("alt", 120)},
			{Conclusion: "d5", EvidenceRef: "r5", Alternatives: strings.Repeat("alt", 120)},
		},
		Artifacts: []agent.ArtifactRef{
			{ID: "a1", Type: "md"}, {ID: "a2", Type: "md"}, {ID: "a3", Type: "md"}, {ID: "a4", Type: "md"}, {ID: "a5", Type: "md"},
		},
		Risks: []agent.Risk{
			{Trigger: "t1", Action: "a1"}, {Trigger: "t2", Action: "a2"}, {Trigger: "t3", Action: "a3"}, {Trigger: "t4", Action: "a4"}, {Trigger: "t5", Action: "a5"},
		},
		EvidenceIndex: []agent.EvidenceRef{
			{Ref: "r1"}, {Ref: "r2"}, {Ref: "r3"}, {Ref: "r4"}, {Ref: "r5"},
			{Ref: "x1"}, {Ref: "x2"}, {Ref: "x3"},
		},
	}

	// Use a tighter limit than default to force compression behavior.
	limit := 500
	compressed, ok := CompressStewardStateForLimit(state, limit)
	if !ok {
		t.Fatalf("expected compression to succeed under limit=%d", limit)
	}
	if compressed == nil {
		t.Fatal("compressed state should not be nil")
	}
	rendered := agent.RenderAsReminder(compressed)
	if len([]rune(rendered)) > limit {
		t.Fatalf("expected rendered chars <= %d, got %d", limit, len([]rune(rendered)))
	}
	if len(compressed.Plan) > 3 {
		t.Fatalf("expected plan to be capped, got %d", len(compressed.Plan))
	}
}

func TestExtractNewState_WithPlanAndTaskGraph(t *testing.T) {
	json := `{
		"version": 5,
		"goal": "Deploy v2",
		"plan": [
			{"input": "run tests", "tool": "bash", "checkpoint": "all green"}
		],
		"task_graph": [
			{"id": "t1", "title": "Build", "status": "done", "depends_on": [], "reversibility": 2},
			{"id": "t2", "title": "Deploy", "status": "pending", "depends_on": ["t1"], "reversibility": 4}
		]
	}`
	content := "<NEW_STATE>" + json + "</NEW_STATE>"

	state, _, err := ExtractNewState(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if len(state.Plan) != 1 {
		t.Errorf("Plan length = %d, want 1", len(state.Plan))
	}
	if state.Plan[0].Tool != "bash" {
		t.Errorf("Plan[0].Tool = %q, want 'bash'", state.Plan[0].Tool)
	}
	if len(state.TaskGraph) != 2 {
		t.Errorf("TaskGraph length = %d, want 2", len(state.TaskGraph))
	}
	if state.TaskGraph[1].DependsOn[0] != "t1" {
		t.Errorf("TaskGraph[1].DependsOn = %v, want [t1]", state.TaskGraph[1].DependsOn)
	}
}

func TestIsStewardStateOversize_False(t *testing.T) {
	if IsStewardStateOversize(nil) {
		t.Error("nil error should not be oversize")
	}
	if IsStewardStateOversize(fmt.Errorf("some other error")) {
		t.Error("unrelated error should not be oversize")
	}
}

func TestRemoveBlock(t *testing.T) {
	tests := []struct {
		name    string
		content string
		start   int
		end     int
		want    string
	}{
		{
			name:    "middle",
			content: "before <TAG>body</TAG> after",
			start:   7,
			end:     22,
			want:    "before\nafter",
		},
		{
			name:    "at end",
			content: "before <TAG>body</TAG>",
			start:   7,
			end:     22,
			want:    "before",
		},
		{
			name:    "at start",
			content: "<TAG>body</TAG> after",
			start:   0,
			end:     15,
			want:    "after",
		},
	}
	for _, tt := range tests {
		got := removeBlock(tt.content, tt.start, tt.end)
		if got != tt.want {
			t.Errorf("%s: removeBlock() = %q, want %q", tt.name, got, tt.want)
		}
	}
}
