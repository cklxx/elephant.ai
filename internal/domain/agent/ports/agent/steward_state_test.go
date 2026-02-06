package agent

import (
	"strings"
	"testing"
)

func TestValidateStewardState_Nil(t *testing.T) {
	if err := ValidateStewardState(nil); err == nil {
		t.Error("expected error for nil state")
	}
}

func TestValidateStewardState_Valid(t *testing.T) {
	s := &StewardState{
		Version: 1,
		Goal:    "Ship feature X",
		Context: "Phase 1",
		Plan: []StewardAction{
			{Input: "search docs", Tool: "web_search"},
		},
		TaskGraph: []TaskGraphNode{
			{ID: "t1", Title: "Research", Status: "done", Reversibility: 1},
		},
	}
	if err := ValidateStewardState(s); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStewardState_TooManyTaskGraphNodes(t *testing.T) {
	s := &StewardState{Version: 1}
	for i := 0; i < MaxTaskGraphNodes+1; i++ {
		s.TaskGraph = append(s.TaskGraph, TaskGraphNode{ID: "n"})
	}
	err := ValidateStewardState(s)
	if err == nil {
		t.Fatal("expected error for too many task graph nodes")
	}
	if !strings.Contains(err.Error(), "task_graph") {
		t.Errorf("error should mention task_graph: %v", err)
	}
}

func TestValidateStewardState_TooManyPlanActions(t *testing.T) {
	s := &StewardState{Version: 1}
	for i := 0; i < MaxPlanActions+1; i++ {
		s.Plan = append(s.Plan, StewardAction{Tool: "t"})
	}
	err := ValidateStewardState(s)
	if err == nil {
		t.Fatal("expected error for too many plan actions")
	}
	if !strings.Contains(err.Error(), "plan") {
		t.Errorf("error should mention plan: %v", err)
	}
}

func TestValidateStewardState_OversizeRendered(t *testing.T) {
	// Fill goal with enough chars to exceed the limit.
	s := &StewardState{
		Version: 1,
		Goal:    strings.Repeat("目标", MaxStewardStateChars),
	}
	err := ValidateStewardState(s)
	if err == nil {
		t.Fatal("expected error for oversize rendered state")
	}
	if !strings.Contains(err.Error(), "chars") {
		t.Errorf("error should mention chars: %v", err)
	}
}

func TestRenderAsReminder_Nil(t *testing.T) {
	if got := RenderAsReminder(nil); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
}

func TestRenderAsReminder_Empty(t *testing.T) {
	s := &StewardState{Version: 1}
	got := RenderAsReminder(s)
	if !strings.Contains(got, "STATE v1") {
		t.Errorf("expected 'STATE v1', got %q", got)
	}
}

func TestRenderAsReminder_FullState(t *testing.T) {
	s := &StewardState{
		Version: 3,
		Goal:    "Launch product",
		Context: "Beta phase",
		Plan: []StewardAction{
			{Input: "write tests", Tool: "code_execute", Checkpoint: "green CI"},
		},
		TaskGraph: []TaskGraphNode{
			{ID: "t1", Title: "Tests", Status: "in_progress", DependsOn: []string{"t0"}, Reversibility: 2},
		},
		Decisions: []Decision{
			{Conclusion: "Use Go", EvidenceRef: "ref-1", Alternatives: "Rust"},
		},
		Artifacts: []ArtifactRef{
			{ID: "a1", Type: "doc", URL: "https://example.com"},
		},
		Risks: []Risk{
			{Trigger: "CI fails", Action: "rollback"},
		},
		EvidenceIndex: []EvidenceRef{
			{Ref: "ref-1", Source: "benchmark", Summary: "Go is 2x faster for this use case"},
		},
	}
	got := RenderAsReminder(s)

	checks := []string{
		"STATE v3",
		"**Goal**: Launch product",
		"**Context**: Beta phase",
		"**Plan**:",
		"[code_execute] write tests",
		"(checkpoint: green CI)",
		"**Task Graph**:",
		"t1: Tests [in_progress] (L2)",
		"← [t0]",
		"**Decisions**:",
		"Use Go [ref:ref-1]",
		"(alt: Rust)",
		"**Artifacts**:",
		"[doc] a1: https://example.com",
		"**Risks**:",
		"IF CI fails → rollback",
		"**Evidence**:",
		"[ref-1] benchmark: Go is 2x faster",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestRenderAsReminder_NoDependencies(t *testing.T) {
	s := &StewardState{
		Version: 1,
		TaskGraph: []TaskGraphNode{
			{ID: "t1", Title: "Solo", Status: "done", Reversibility: 1},
		},
	}
	got := RenderAsReminder(s)
	// Should not contain dependency arrow when DependsOn is empty.
	if strings.Contains(got, "←") {
		t.Errorf("unexpected dependency arrow in output:\n%s", got)
	}
}

func TestRenderAsReminder_NoCheckpoint(t *testing.T) {
	s := &StewardState{
		Version: 1,
		Plan: []StewardAction{
			{Input: "search", Tool: "web_search"},
		},
	}
	got := RenderAsReminder(s)
	if strings.Contains(got, "checkpoint") {
		t.Errorf("unexpected checkpoint in output:\n%s", got)
	}
}
