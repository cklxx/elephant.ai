package context

import (
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestBuildEnvelopeBindsUserAndBudgets(t *testing.T) {
	window := ports.ContextWindow{
		SessionID:    "sess-1",
		SystemPrompt: strings.Repeat("system", 10),
		Static: ports.StaticContext{
			Persona: ports.PersonaProfile{ID: "persona-a"},
		},
		Dynamic: ports.DynamicContext{TurnID: 3},
		Meta:    ports.MetaContext{PersonaVersion: "v1"},
	}
	session := &ports.Session{
		ID: "sess-1",
		Metadata: map[string]string{
			"user_id": "user-777",
		},
	}

	budgets := SectionBudgets{
		Threshold: 0.8,
		System:    SectionBudget{Limit: 2},
		Static:    SectionBudget{Limit: 1},
		Dynamic:   SectionBudget{Limit: 1},
		Meta:      SectionBudget{Limit: 1},
	}

	env := BuildEnvelope(window, session, budgets)

	if env.UserRef != "user-777" {
		t.Fatalf("expected user ref to bind metadata id, got %q", env.UserRef)
	}
	if env.SessionID != window.SessionID {
		t.Fatalf("expected session id passthrough, got %q", env.SessionID)
	}
	if env.Hash == "" {
		t.Fatalf("expected hash to be populated")
	}

	if !env.Budgets[SectionSystem].ShouldCompress {
		t.Fatalf("expected system section to exceed budget and request compression")
	}
	if env.Budgets[SectionStatic].Limit != 1 {
		t.Fatalf("static budget not preserved: %+v", env.Budgets[SectionStatic])
	}
}

func TestBindUserRefFallsBackToSessionID(t *testing.T) {
	session := &ports.Session{ID: "sess-22"}
	if ref := bindUserRef(session); ref != "sess-22" {
		t.Fatalf("expected fallback to session id, got %q", ref)
	}
}

func TestBudgetStatusUsesDefaultThreshold(t *testing.T) {
	status := buildBudgetStatus(SectionBudget{Limit: 10}, 7, 0)
	if status.Threshold != defaultThreshold {
		t.Fatalf("expected default threshold %v, got %v", defaultThreshold, status.Threshold)
	}
	if status.ShouldCompress {
		t.Fatalf("threshold should not trigger compression at 7/10")
	}
	status = buildBudgetStatus(SectionBudget{Limit: 10}, 9, 0.5)
	if !status.ShouldCompress {
		t.Fatalf("custom threshold 0.5 should trigger compression")
	}
}

func TestEnvelopeBudgetSummaries(t *testing.T) {
	env := Envelope{
		TokensBySection: map[SectionName]int{
			SectionSystem:  5,
			SectionDynamic: 2,
		},
		Budgets: map[SectionName]SectionBudgetStatus{
			SectionSystem: {
				Limit:          2,
				Threshold:      0.8,
				Tokens:         5,
				ShouldCompress: true,
			},
			SectionDynamic: {
				Limit:          5,
				Threshold:      0.8,
				Tokens:         2,
				ShouldCompress: false,
			},
		},
	}

	budgets := env.BudgetSummaries()
	if len(budgets) != 2 {
		t.Fatalf("expected 2 budget summaries, got %d", len(budgets))
	}

	summaries := make(map[string]ports.SectionBudgetStatus)
	for _, b := range budgets {
		summaries[b.Section] = b
	}
	if !summaries["system"].ShouldCompress {
		t.Fatalf("expected system section to request compression")
	}
	if summaries["dynamic"].Tokens != 2 {
		t.Fatalf("expected dynamic tokens to be preserved, got %+v", summaries["dynamic"])
	}

	tokens := env.TokenSummaries()
	if tokens["system"] != 5 || tokens["dynamic"] != 2 {
		t.Fatalf("unexpected token normalization: %+v", tokens)
	}
}
