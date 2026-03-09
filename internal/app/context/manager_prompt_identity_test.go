package context

import (
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestBuildIdentitySectionIncludesPersonaAttributes(t *testing.T) {
	section := buildIdentitySection(agent.PersonaProfile{
		ID:            "default",
		Voice:         "You are eli.",
		Tone:          "pragmatic",
		DecisionStyle: "evidence-first",
		RiskProfile:   "calibrated",
	})

	for _, snippet := range []string{
		"You are eli.",
		"Tone: pragmatic",
		"Decision Style: evidence-first",
		"Risk Profile: calibrated",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected identity section to contain %q, got: %s", snippet, section)
		}
	}
}
