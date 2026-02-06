package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/skills"
	"alex/internal/shared/logging"
)

func TestBuildSkillsSection_FallbackDoesNotInjectCatalog(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "foo-skill", "Foo workflow.", `
triggers:
  intent_patterns:
    - foo
  confidence_threshold: 0.5
`, "# Foo Skill\n\nDo foo.\n")
	writeSkillFile(t, dir, "bar-skill", "Bar workflow.", "", "# Bar Skill\n\nDo bar.\n")

	t.Setenv("ALEX_SKILLS_DIR", dir)
	skills.InvalidateCache()

	cfg := agent.SkillsConfig{
		AutoActivation: agent.SkillAutoActivationConfig{
			Enabled:             true,
			MaxActivated:        3,
			TokenBudget:         10_000,
			ConfidenceThreshold: 0.5,
			CacheTTLSeconds:     0,
			FallbackToIndex:     true,
		},
	}

	out := buildSkillsSection(logging.Nop(), "please foo", nil, "session-1", cfg)
	if out == "" {
		t.Fatalf("expected non-empty skills section")
	}
	if !strings.Contains(out, "<available_skills>") {
		t.Fatalf("expected available skills metadata, got %q", out)
	}
	if !strings.Contains(out, "<name>foo-skill</name>") || !strings.Contains(out, "<name>bar-skill</name>") {
		t.Fatalf("expected all skill names in metadata, got %q", out)
	}
	if !strings.Contains(out, "# Activated Skills") {
		t.Fatalf("expected activated skills section, got %q", out)
	}
	if !strings.Contains(out, "## Skill: foo-skill") {
		t.Fatalf("expected foo-skill to be activated, got %q", out)
	}
	if !strings.Contains(out, "Do foo") {
		t.Fatalf("expected skill body to be rendered, got %q", out)
	}
	if !strings.Contains(out, "# Skill Discovery") {
		t.Fatalf("expected skill discovery hint, got %q", out)
	}
}

func TestBuildSkillsSection_NoMatchStillAvoidsCatalog(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "sample-skill", "Sample workflow.", "", "# Sample Skill\n\nBody.\n")

	t.Setenv("ALEX_SKILLS_DIR", dir)
	skills.InvalidateCache()

	cfg := agent.SkillsConfig{
		AutoActivation: agent.SkillAutoActivationConfig{
			Enabled:             true,
			MaxActivated:        3,
			TokenBudget:         10_000,
			ConfidenceThreshold: 0.5,
			CacheTTLSeconds:     0,
			FallbackToIndex:     true,
		},
	}

	out := buildSkillsSection(logging.Nop(), "no match here", nil, "session-1", cfg)
	if out == "" {
		t.Fatalf("expected non-empty skills section")
	}
	if !strings.Contains(out, "<available_skills>") {
		t.Fatalf("expected available skills metadata, got %q", out)
	}
	if strings.Contains(out, "## Skill: sample-skill") || strings.Contains(out, "# Activated Skills") {
		t.Fatalf("did not expect activated skills to be rendered, got %q", out)
	}
	if !strings.Contains(out, "# Skill Discovery") {
		t.Fatalf("expected skill discovery hint, got %q", out)
	}
}

func writeSkillFile(t *testing.T, root string, name string, description string, extraFrontMatter string, body string) {
	t.Helper()

	content := strings.TrimSpace("---\nname: "+name+"\ndescription: "+description+"\n"+strings.TrimSpace(extraFrontMatter)+"\n---\n\n"+strings.TrimSpace(body)) + "\n"
	skillDir := filepath.Join(root, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}
