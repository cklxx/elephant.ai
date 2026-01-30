package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillMatcherIntentPattern(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: deploy-skill
description: Deploy workflow.
triggers:
  intent_patterns:
    - "deploy|release"
  confidence_threshold: 0.6
priority: 5
---
# Deploy Skill
`
	if err := os.WriteFile(filepath.Join(dir, "deploy.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	matcher := NewSkillMatcher(&lib, MatcherOptions{})
	matches := matcher.Match(MatchContext{
		TaskInput: "Please deploy the service",
		SessionID: "sess-1",
	}, AutoActivationConfig{Enabled: true, ConfidenceThreshold: 0.5})

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Skill.Name != "deploy-skill" {
		t.Fatalf("expected deploy-skill, got %s", matches[0].Skill.Name)
	}
}

func TestSkillMatcherExclusiveGroup(t *testing.T) {
	dir := t.TempDir()
	skillA := `---
name: skill-a
description: A
triggers:
  intent_patterns:
    - "report"
priority: 5
exclusive_group: reports
---
# Skill A
`
	skillB := `---
name: skill-b
description: B
triggers:
  intent_patterns:
    - "report"
priority: 8
exclusive_group: reports
---
# Skill B
`
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(skillA), 0o644); err != nil {
		t.Fatalf("write skill A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte(skillB), 0o644); err != nil {
		t.Fatalf("write skill B: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	matcher := NewSkillMatcher(&lib, MatcherOptions{})
	matches := matcher.Match(MatchContext{
		TaskInput: "generate a report",
		SessionID: "sess-2",
	}, AutoActivationConfig{Enabled: true, ConfidenceThreshold: 0.4})

	if len(matches) != 1 {
		t.Fatalf("expected 1 match after conflict resolution, got %d", len(matches))
	}
	if matches[0].Skill.Name != "skill-b" {
		t.Fatalf("expected skill-b to win, got %s", matches[0].Skill.Name)
	}
}

func TestApplyActivationLimitsRespectsBudget(t *testing.T) {
	matches := []MatchResult{
		{Skill: Skill{Name: "a", Body: "one two three four", MaxTokens: 4}},
		{Skill: Skill{Name: "b", Body: "one two three four five six", MaxTokens: 6}},
	}
	out := ApplyActivationLimits(matches, AutoActivationConfig{TokenBudget: 4})
	if len(out) != 1 {
		t.Fatalf("expected 1 match within budget, got %d", len(out))
	}
	if out[0].Skill.Name != "a" {
		t.Fatalf("expected skill a, got %s", out[0].Skill.Name)
	}
}
