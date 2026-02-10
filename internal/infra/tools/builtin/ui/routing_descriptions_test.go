package ui

import (
	"strings"
	"testing"
)

func TestUIDescriptionsExpressPlanClarifyRequestBoundaries(t *testing.T) {
	t.Parallel()

	planDesc := NewPlan(nil).Definition().Description
	if !strings.Contains(planDesc, "decomposition, phases, milestones, checkpoints") || !strings.Contains(planDesc, "Do not use for deterministic computation/recalculation") {
		t.Fatalf("expected plan description to mention decomposition-vs-computation boundary, got %q", planDesc)
	}

	clarifyDesc := NewClarify().Definition().Description
	if !strings.Contains(clarifyDesc, "requirements are truly missing/contradictory") || !strings.Contains(clarifyDesc, "Do not use for approval/consent/manual confirmation gates") {
		t.Fatalf("expected clarify description to mention requirement-vs-approval boundary, got %q", clarifyDesc)
	}

	requestDesc := NewRequestUser().Definition().Description
	if !strings.Contains(requestDesc, "approval/consent/manual gate events") || !strings.Contains(requestDesc, "Do not use for internal planning headers") {
		t.Fatalf("expected request_user description to mention approval-vs-planning boundary, got %q", requestDesc)
	}
}
