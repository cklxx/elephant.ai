package context

import (
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestBuildIdentitySectionIncludesIdentityFileLocations(t *testing.T) {
	section := buildIdentitySection(agent.PersonaProfile{
		ID:            "default",
		Voice:         "You are eli.",
		Tone:          "pragmatic",
		DecisionStyle: "evidence-first",
		RiskProfile:   "calibrated",
	})

	if !strings.Contains(section, "SOUL.md: ~/.alex/memory/SOUL.md") {
		t.Fatalf("expected SOUL.md path hint in identity section, got: %s", section)
	}
	if !strings.Contains(section, "USER.md: ~/.alex/memory/USER.md") {
		t.Fatalf("expected USER.md path hint in identity section, got: %s", section)
	}
	if !strings.Contains(section, "docs/reference/SOUL.md") {
		t.Fatalf("expected persona source path hint in identity section, got: %s", section)
	}
}
