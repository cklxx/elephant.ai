package ui

import (
	"strings"
	"testing"
)

func TestUIDescriptionsExpressPlanAskUserBoundaries(t *testing.T) {
	t.Parallel()

	planDesc := NewPlan(nil).Definition().Description
	if !strings.Contains(planDesc, "decomposition, phases, milestones, checkpoints") || !strings.Contains(planDesc, "Do not use for deterministic computation/recalculation") {
		t.Fatalf("expected plan description to mention decomposition-vs-computation boundary, got %q", planDesc)
	}

	askUserDesc := NewAskUser().Definition().Description
	if !strings.Contains(askUserDesc, "clarification questions") || !strings.Contains(askUserDesc, "request a user decision/action") {
		t.Fatalf("expected ask_user description to cover both clarify and request actions, got %q", askUserDesc)
	}
}
