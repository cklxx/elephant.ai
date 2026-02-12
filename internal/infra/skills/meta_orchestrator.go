package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MetaPolicy configures meta-level skill orchestration behavior.
type MetaPolicy struct {
	Orchestrator MetaOrchestratorPolicy `yaml:"orchestrator"`
	Governance   MetaGovernancePolicy   `yaml:"governance"`
	Links        []SkillLinkRule        `yaml:"links"`
}

// MetaOrchestratorPolicy controls orchestration behavior defaults.
type MetaOrchestratorPolicy struct {
	DefaultActivationMode string `yaml:"default_activation_mode"`
	MaxLinkedSkills       int    `yaml:"max_linked_skills"`
	ProactiveLevel        string `yaml:"proactive_level"`
}

// MetaGovernancePolicy controls meta governance behavior.
type MetaGovernancePolicy struct {
	BlockedGovernanceLevels []string `yaml:"blocked_governance_levels"`
	RequireApprovalLevels   []string `yaml:"require_approval_levels"`
	ImmutableSoulSections   []string `yaml:"immutable_soul_sections"`
}

// SkillLinkRule declares a linkage between two skills.
type SkillLinkRule struct {
	From    string `yaml:"from"`
	To      string `yaml:"to"`
	OnEvent string `yaml:"on_event"`
}

// OrchestrationConfig controls runtime orchestration toggles.
type OrchestrationConfig struct {
	Enabled                  bool
	SoulAutoEvolutionEnabled bool
	ProactiveLevel           string
}

// BlockedSkill captures an activation blocked by orchestration policy.
type BlockedSkill struct {
	Name   string
	Reason string
}

// OrchestrationPlan is the final deterministic skill activation plan.
type OrchestrationPlan struct {
	Selected       []MatchResult
	Blocked        []BlockedSkill
	Links          []SkillLinkRule
	RiskLevel      string
	ProactiveLevel string
	Events         []string
	Policy         MetaPolicy
}

// DefaultMetaPolicy returns conservative policy defaults.
func DefaultMetaPolicy() MetaPolicy {
	return MetaPolicy{
		Orchestrator: MetaOrchestratorPolicy{
			DefaultActivationMode: "auto",
			MaxLinkedSkills:       4,
			ProactiveLevel:        "medium",
		},
		Governance: MetaGovernancePolicy{
			RequireApprovalLevels: []string{"critical"},
		},
	}
}

// LoadMetaPolicy loads orchestration policy YAML from path.
// Missing paths fall back to defaults.
func LoadMetaPolicy(path string) (MetaPolicy, error) {
	policy := DefaultMetaPolicy()
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return policy, nil
	}
	data, err := os.ReadFile(trimmed)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return policy, nil
		}
		return policy, fmt.Errorf("read meta policy %s: %w", trimmed, err)
	}
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return policy, fmt.Errorf("parse meta policy %s: %w", trimmed, err)
	}
	policy.Orchestrator.DefaultActivationMode = normalizeActivationMode(policy.Orchestrator.DefaultActivationMode)
	policy.Orchestrator.ProactiveLevel = normalizeProactiveLevel(policy.Orchestrator.ProactiveLevel)
	if policy.Orchestrator.MaxLinkedSkills <= 0 {
		policy.Orchestrator.MaxLinkedSkills = 4
	}
	return policy, nil
}

// OrchestrateMatches filters, orders, and links skills using runtime config + policy.
func OrchestrateMatches(matches []MatchResult, cfg OrchestrationConfig, policy MetaPolicy) OrchestrationPlan {
	plan := OrchestrationPlan{
		Policy: policy,
	}
	if len(matches) == 0 {
		plan.ProactiveLevel = chooseProactiveLevel(cfg.ProactiveLevel, policy.Orchestrator.ProactiveLevel)
		plan.RiskLevel = "low"
		return plan
	}
	if !cfg.Enabled {
		plan.Selected = append([]MatchResult(nil), matches...)
		plan.ProactiveLevel = chooseProactiveLevel(cfg.ProactiveLevel, policy.Orchestrator.ProactiveLevel)
		plan.RiskLevel = highestGovernanceLevel(matches)
		plan.Events = collectProducedEvents(matches)
		return plan
	}

	proactiveLevel := chooseProactiveLevel(cfg.ProactiveLevel, policy.Orchestrator.ProactiveLevel)
	plan.ProactiveLevel = proactiveLevel
	blockedLevels := makeSet(policy.Governance.BlockedGovernanceLevels)
	requireApprovalLevels := makeSet(policy.Governance.RequireApprovalLevels)
	defaultMode := normalizeActivationMode(policy.Orchestrator.DefaultActivationMode)

	selected := make([]MatchResult, 0, len(matches))
	blocked := make([]BlockedSkill, 0, len(matches))
	for _, match := range matches {
		skill := match.Skill
		mode := normalizeActivationMode(skill.ActivationMode)
		if mode == "" {
			mode = defaultMode
		}
		if mode == "manual" {
			blocked = append(blocked, BlockedSkill{Name: skill.Name, Reason: "activation_mode=manual"})
			continue
		}
		level := normalizeGovernanceLevel(skill.GovernanceLevel)
		if blockedLevels[level] {
			blocked = append(blocked, BlockedSkill{Name: skill.Name, Reason: "blocked governance level"})
			continue
		}
		if !cfg.SoulAutoEvolutionEnabled && hasCapability(skill, "self_evolve_soul") {
			blocked = append(blocked, BlockedSkill{Name: skill.Name, Reason: "soul auto evolution disabled"})
			continue
		}
		if skill.RequiresApproval && requireApprovalLevels[level] {
			blocked = append(blocked, BlockedSkill{Name: skill.Name, Reason: "requires approval by governance policy"})
			continue
		}
		selected = append(selected, match)
	}

	maxByProactive := proactiveActivationCap(proactiveLevel)
	if maxByProactive > 0 && len(selected) > maxByProactive {
		for _, dropped := range selected[maxByProactive:] {
			blocked = append(blocked, BlockedSkill{Name: dropped.Skill.Name, Reason: "proactive cap reached"})
		}
		selected = selected[:maxByProactive]
	}

	selected = orderByDependencies(selected)
	plan.Selected = selected
	plan.Blocked = blocked
	plan.Links = selectSkillLinks(policy.Links, selected, policy.Orchestrator.MaxLinkedSkills)
	plan.RiskLevel = highestGovernanceLevel(selected)
	plan.Events = collectProducedEvents(selected)
	return plan
}

// RenderOrchestrationSummary renders compact orchestration details for prompt context.
func RenderOrchestrationSummary(plan OrchestrationPlan) string {
	var sb strings.Builder
	sb.WriteString("# Meta Skill Orchestration\n\n")
	sb.WriteString(fmt.Sprintf("- Proactive level: %s\n", plan.ProactiveLevel))
	sb.WriteString(fmt.Sprintf("- Risk level: %s\n", plan.RiskLevel))
	if len(plan.Selected) == 0 {
		sb.WriteString("- Selected skills: none\n")
	} else {
		sb.WriteString(fmt.Sprintf("- Selected skills: %s\n", joinSkillNames(plan.Selected)))
	}
	if len(plan.Blocked) > 0 {
		parts := make([]string, 0, len(plan.Blocked))
		for _, blocked := range plan.Blocked {
			parts = append(parts, fmt.Sprintf("%s (%s)", blocked.Name, blocked.Reason))
		}
		sb.WriteString(fmt.Sprintf("- Blocked skills: %s\n", strings.Join(parts, "; ")))
	}
	if len(plan.Links) > 0 {
		linkParts := make([]string, 0, len(plan.Links))
		for _, link := range plan.Links {
			evt := strings.TrimSpace(link.OnEvent)
			if evt == "" {
				linkParts = append(linkParts, fmt.Sprintf("%s -> %s", link.From, link.To))
				continue
			}
			linkParts = append(linkParts, fmt.Sprintf("%s -> %s (%s)", link.From, link.To, evt))
		}
		sb.WriteString(fmt.Sprintf("- Linked flows: %s\n", strings.Join(linkParts, "; ")))
	}
	if len(plan.Events) > 0 {
		sb.WriteString(fmt.Sprintf("- Declared events: %s\n", strings.Join(plan.Events, ", ")))
	}
	if len(plan.Policy.Governance.ImmutableSoulSections) > 0 {
		sb.WriteString(fmt.Sprintf("- Immutable SOUL sections: %s\n", strings.Join(plan.Policy.Governance.ImmutableSoulSections, ", ")))
	}
	return strings.TrimSpace(sb.String())
}

func orderByDependencies(selected []MatchResult) []MatchResult {
	if len(selected) <= 1 {
		return selected
	}
	index := make(map[string]MatchResult, len(selected))
	for _, match := range selected {
		index[NormalizeName(match.Skill.Name)] = match
	}

	visiting := make(map[string]bool, len(selected))
	visited := make(map[string]bool, len(selected))
	ordered := make([]MatchResult, 0, len(selected))
	var visit func(name string)
	visit = func(name string) {
		if visited[name] || visiting[name] {
			return
		}
		visiting[name] = true
		match, ok := index[name]
		if ok {
			for _, dep := range match.Skill.DependsOnSkills {
				norm := NormalizeName(dep)
				if _, exists := index[norm]; exists {
					visit(norm)
				}
			}
		}
		visiting[name] = false
		visited[name] = true
		if ok {
			ordered = append(ordered, match)
		}
	}

	for _, match := range selected {
		visit(NormalizeName(match.Skill.Name))
	}
	return ordered
}

func selectSkillLinks(rules []SkillLinkRule, selected []MatchResult, max int) []SkillLinkRule {
	if len(rules) == 0 || len(selected) == 0 || max == 0 {
		return nil
	}
	selectedSet := make(map[string]struct{}, len(selected))
	for _, match := range selected {
		selectedSet[NormalizeName(match.Skill.Name)] = struct{}{}
	}
	out := make([]SkillLinkRule, 0, min(len(rules), max))
	for _, rule := range rules {
		from := NormalizeName(rule.From)
		to := NormalizeName(rule.To)
		if from == "" || to == "" {
			continue
		}
		if _, ok := selectedSet[from]; !ok {
			continue
		}
		if _, ok := selectedSet[to]; !ok {
			continue
		}
		out = append(out, SkillLinkRule{
			From:    from,
			To:      to,
			OnEvent: strings.TrimSpace(rule.OnEvent),
		})
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func proactiveActivationCap(level string) int {
	switch normalizeProactiveLevel(level) {
	case "low":
		return 2
	case "high":
		return 5
	default:
		return 3
	}
}

func chooseProactiveLevel(override string, fallback string) string {
	level := normalizeProactiveLevel(override)
	if level != "" {
		return level
	}
	level = normalizeProactiveLevel(fallback)
	if level != "" {
		return level
	}
	return "medium"
}

func normalizeProactiveLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return ""
	}
}

func normalizeActivationMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto", "semi_auto", "manual":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ""
	}
}

func normalizeGovernanceLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "medium"
	}
}

func highestGovernanceLevel(matches []MatchResult) string {
	if len(matches) == 0 {
		return "low"
	}
	rank := map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}
	best := "low"
	bestRank := 0
	for _, match := range matches {
		level := normalizeGovernanceLevel(match.Skill.GovernanceLevel)
		if rank[level] > bestRank {
			bestRank = rank[level]
			best = level
		}
	}
	return best
}

func joinSkillNames(matches []MatchResult) string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match.Skill.Name)
	}
	return strings.Join(names, ", ")
}

func hasCapability(skill Skill, capability string) bool {
	target := strings.ToLower(strings.TrimSpace(capability))
	if target == "" {
		return false
	}
	for _, item := range skill.Capabilities {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func collectProducedEvents(matches []MatchResult) []string {
	set := make(map[string]struct{})
	for _, match := range matches {
		for _, evt := range match.Skill.ProducesEvents {
			trimmed := strings.TrimSpace(evt)
			if trimmed == "" {
				continue
			}
			set[trimmed] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for evt := range set {
		out = append(out, evt)
	}
	sort.Strings(out)
	return out
}

func makeSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		out[key] = true
	}
	return out
}
