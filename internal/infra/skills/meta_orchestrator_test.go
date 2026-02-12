package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMetaPolicyMissingFileFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	policy, err := LoadMetaPolicy(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("load meta policy: %v", err)
	}
	if policy.Orchestrator.DefaultActivationMode != "auto" {
		t.Fatalf("expected default activation mode auto, got %q", policy.Orchestrator.DefaultActivationMode)
	}
	if policy.Orchestrator.ProactiveLevel != "medium" {
		t.Fatalf("expected default proactive level medium, got %q", policy.Orchestrator.ProactiveLevel)
	}
}

func TestOrchestrateMatchesFiltersAndLinks(t *testing.T) {
	t.Parallel()

	matches := []MatchResult{
		{
			Skill: Skill{
				Name:            "meta-orchestrator",
				ActivationMode:  "auto",
				GovernanceLevel: "high",
				Capabilities:    []string{"orchestrate_skills"},
				ProducesEvents:  []string{"workflow.skill.meta.route_selected"},
			},
			Score: 0.9,
		},
		{
			Skill: Skill{
				Name:            "lark-conversation-governor",
				ActivationMode:  "auto",
				GovernanceLevel: "medium",
				DependsOnSkills: []string{"meta-orchestrator"},
				Capabilities:    []string{"lark_chat"},
				ProducesEvents:  []string{"workflow.skill.meta.link_executed"},
			},
			Score: 0.8,
		},
		{
			Skill: Skill{
				Name:            "soul-self-evolution",
				ActivationMode:  "auto",
				GovernanceLevel: "critical",
				Capabilities:    []string{"self_evolve_soul"},
				ProducesEvents:  []string{"workflow.skill.meta.soul_updated"},
			},
			Score: 0.7,
		},
	}

	policy := MetaPolicy{
		Orchestrator: MetaOrchestratorPolicy{
			DefaultActivationMode: "auto",
			MaxLinkedSkills:       5,
			ProactiveLevel:        "high",
		},
		Governance: MetaGovernancePolicy{
			RequireApprovalLevels: []string{"critical"},
		},
		Links: []SkillLinkRule{
			{From: "meta-orchestrator", To: "lark-conversation-governor", OnEvent: "workflow.skill.meta.route_selected"},
		},
	}
	plan := OrchestrateMatches(matches, OrchestrationConfig{
		Enabled:                  true,
		SoulAutoEvolutionEnabled: false,
		ProactiveLevel:           "medium",
	}, policy)

	if len(plan.Selected) != 2 {
		t.Fatalf("expected 2 selected skills, got %d", len(plan.Selected))
	}
	if plan.Selected[0].Skill.Name != "meta-orchestrator" || plan.Selected[1].Skill.Name != "lark-conversation-governor" {
		t.Fatalf("expected dependency order, got %s -> %s", plan.Selected[0].Skill.Name, plan.Selected[1].Skill.Name)
	}
	if len(plan.Blocked) != 1 || plan.Blocked[0].Name != "soul-self-evolution" {
		t.Fatalf("expected soul skill blocked, got %+v", plan.Blocked)
	}
	if len(plan.Links) != 1 {
		t.Fatalf("expected one link, got %+v", plan.Links)
	}
	if plan.RiskLevel != "high" {
		t.Fatalf("expected risk high, got %q", plan.RiskLevel)
	}
	if !strings.Contains(RenderOrchestrationSummary(plan), "Meta Skill Orchestration") {
		t.Fatalf("expected orchestration summary header")
	}
}

func TestLoadMetaPolicyParsesCustomPolicy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "meta-orchestrator.yaml")
	content := `orchestrator:
  default_activation_mode: semi_auto
  max_linked_skills: 2
  proactive_level: high
governance:
  blocked_governance_levels: [critical]
  require_approval_levels: [high]
  immutable_soul_sections:
    - "## Core Identity"
links:
  - from: "meta-orchestrator"
    to: "autonomous-scheduler"
    on_event: "workflow.skill.meta.route_selected"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	policy, err := LoadMetaPolicy(path)
	if err != nil {
		t.Fatalf("load meta policy: %v", err)
	}
	if policy.Orchestrator.DefaultActivationMode != "semi_auto" {
		t.Fatalf("unexpected activation mode: %q", policy.Orchestrator.DefaultActivationMode)
	}
	if len(policy.Governance.ImmutableSoulSections) != 1 {
		t.Fatalf("expected immutable soul section, got %+v", policy.Governance.ImmutableSoulSections)
	}
	if len(policy.Links) != 1 || policy.Links[0].To != "autonomous-scheduler" {
		t.Fatalf("unexpected links: %+v", policy.Links)
	}
}
