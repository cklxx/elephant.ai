package agent_eval

import (
	"errors"
	"fmt"
	"time"
)

// DifficultyTier represents hierarchical difficulty levels for evaluation tasks.
type DifficultyTier string

const (
	TierEasy   DifficultyTier = "easy"
	TierMedium DifficultyTier = "medium"
	TierHard   DifficultyTier = "hard"
	TierExpert DifficultyTier = "expert"
)

// EvalDomain represents the domain stratification of evaluation tasks.
type EvalDomain string

const (
	DomainReasoning    EvalDomain = "reasoning"
	DomainToolUse      EvalDomain = "tool_use"
	DomainCodeGen      EvalDomain = "code_gen"
	DomainRetrieval    EvalDomain = "retrieval"
	DomainConversation EvalDomain = "conversation"
	DomainPlanning     EvalDomain = "planning"
)

// EvalTask represents a single evaluation task with rich metadata for
// difficulty-stratified, domain-aware evaluation sets.
type EvalTask struct {
	ID             string         `json:"id" yaml:"id"`
	Title          string         `json:"title" yaml:"title"`
	Goal           string         `json:"goal" yaml:"goal"`
	Difficulty     DifficultyTier `json:"difficulty" yaml:"difficulty"`
	Domain         EvalDomain     `json:"domain" yaml:"domain"`
	Tags           []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
	ExpectedSteps  int            `json:"expected_steps,omitempty" yaml:"expected_steps,omitempty"`
	MaxTokenBudget int            `json:"max_token_budget,omitempty" yaml:"max_token_budget,omitempty"`
	PassCriteria   []string       `json:"pass_criteria,omitempty" yaml:"pass_criteria,omitempty"`
	Weight         float64        `json:"weight" yaml:"weight"`
}

// effectiveWeight returns the task weight, defaulting to 1.0 when zero.
func (t EvalTask) effectiveWeight() float64 {
	if t.Weight == 0 {
		return 1.0
	}
	return t.Weight
}

// EvalSetConfig holds versioned configuration and composition rules for an eval set.
type EvalSetConfig struct {
	Version          string            `json:"version" yaml:"version"`
	Name             string            `json:"name" yaml:"name"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
	CompositionRules []CompositionRule `json:"composition_rules,omitempty" yaml:"composition_rules,omitempty"`
}

// CompositionRule defines a distribution constraint within an eval set.
// Empty Domain/Difficulty/Tags means "any" for that dimension.
type CompositionRule struct {
	Domain     EvalDomain     `json:"domain,omitempty" yaml:"domain,omitempty"`
	Difficulty DifficultyTier `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	Tags       []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
	Percentage float64        `json:"percentage" yaml:"percentage"`
	MinCount   int            `json:"min_count" yaml:"min_count"`
}

// matches reports whether a task satisfies this rule's filters.
func (r CompositionRule) matches(task EvalTask) bool {
	if r.Domain != "" && task.Domain != r.Domain {
		return false
	}
	if r.Difficulty != "" && task.Difficulty != r.Difficulty {
		return false
	}
	if len(r.Tags) > 0 && !hasAnyTag(task.Tags, r.Tags) {
		return false
	}
	return true
}

// EvalSet is a versioned, composable collection of evaluation tasks.
type EvalSet struct {
	Config    EvalSetConfig `json:"config" yaml:"config"`
	Tasks     []EvalTask    `json:"tasks" yaml:"tasks"`
	CreatedAt time.Time     `json:"created_at" yaml:"created_at"`
}

// TasksByDifficulty groups the set's tasks by their difficulty tier.
func (s *EvalSet) TasksByDifficulty() map[DifficultyTier][]EvalTask {
	result := make(map[DifficultyTier][]EvalTask)
	for _, t := range s.Tasks {
		result[t.Difficulty] = append(result[t.Difficulty], t)
	}
	return result
}

// TasksByDomain groups the set's tasks by their evaluation domain.
func (s *EvalSet) TasksByDomain() map[EvalDomain][]EvalTask {
	result := make(map[EvalDomain][]EvalTask)
	for _, t := range s.Tasks {
		result[t.Domain] = append(result[t.Domain], t)
	}
	return result
}

// Summary computes aggregate statistics for the eval set.
func (s *EvalSet) Summary() EvalSetSummary {
	summary := EvalSetSummary{
		TotalTasks:   len(s.Tasks),
		ByDifficulty: make(map[DifficultyTier]int),
		ByDomain:     make(map[EvalDomain]int),
		Version:      s.Config.Version,
	}

	var totalWeight float64
	for _, t := range s.Tasks {
		summary.ByDifficulty[t.Difficulty]++
		summary.ByDomain[t.Domain]++
		totalWeight += t.effectiveWeight()
	}

	if len(s.Tasks) > 0 {
		summary.AvgWeight = totalWeight / float64(len(s.Tasks))
	}

	return summary
}

// EvalSetSummary holds aggregate statistics for an EvalSet.
type EvalSetSummary struct {
	TotalTasks   int                    `json:"total_tasks" yaml:"total_tasks"`
	ByDifficulty map[DifficultyTier]int `json:"by_difficulty" yaml:"by_difficulty"`
	ByDomain     map[EvalDomain]int     `json:"by_domain" yaml:"by_domain"`
	AvgWeight    float64                `json:"avg_weight" yaml:"avg_weight"`
	Version      string                 `json:"version" yaml:"version"`
}

// ValidateComposition checks composition rules against the eval set and returns
// a list of human-readable violation strings. An empty slice means all rules pass.
func ValidateComposition(set *EvalSet) []string {
	if set == nil {
		return []string{"eval set is nil"}
	}

	var violations []string
	total := len(set.Tasks)

	for i, rule := range set.Config.CompositionRules {
		matching := 0
		for _, t := range set.Tasks {
			if rule.matches(t) {
				matching++
			}
		}

		// Check MinCount constraint.
		if rule.MinCount > 0 && matching < rule.MinCount {
			violations = append(violations, fmt.Sprintf(
				"rule[%d]: expected min %d tasks, got %d",
				i, rule.MinCount, matching,
			))
		}

		// Check Percentage constraint.
		if rule.Percentage > 0 && total > 0 {
			actualPct := float64(matching) / float64(total)
			if actualPct < rule.Percentage {
				violations = append(violations, fmt.Sprintf(
					"rule[%d]: expected >= %.1f%% of tasks, got %.1f%%",
					i, rule.Percentage*100, actualPct*100,
				))
			}
		}
	}

	return violations
}

// ---------------------------------------------------------------------------
// EvalSetBuilder — fluent builder for constructing EvalSet instances
// ---------------------------------------------------------------------------

// EvalSetBuilder provides a fluent API for constructing validated EvalSets.
type EvalSetBuilder struct {
	name        string
	version     string
	description string
	tasks       []EvalTask
	rules       []CompositionRule

	// filters applied during Build
	filterDifficulty *DifficultyTier
	filterDomain     *EvalDomain
	filterTags       []string
	limit            int
}

// NewEvalSetBuilder creates a builder with the required name and version.
func NewEvalSetBuilder(name, version string) *EvalSetBuilder {
	return &EvalSetBuilder{
		name:    name,
		version: version,
	}
}

// WithDescription sets the eval set description.
func (b *EvalSetBuilder) WithDescription(desc string) *EvalSetBuilder {
	b.description = desc
	return b
}

// AddTask appends a single task to the builder.
func (b *EvalSetBuilder) AddTask(task EvalTask) *EvalSetBuilder {
	b.tasks = append(b.tasks, task)
	return b
}

// AddTasks appends multiple tasks to the builder.
func (b *EvalSetBuilder) AddTasks(tasks []EvalTask) *EvalSetBuilder {
	b.tasks = append(b.tasks, tasks...)
	return b
}

// WithCompositionRule adds a composition rule to the eval set config.
func (b *EvalSetBuilder) WithCompositionRule(rule CompositionRule) *EvalSetBuilder {
	b.rules = append(b.rules, rule)
	return b
}

// FilterByDifficulty restricts the built set to tasks matching the given tier.
func (b *EvalSetBuilder) FilterByDifficulty(tier DifficultyTier) *EvalSetBuilder {
	b.filterDifficulty = &tier
	return b
}

// FilterByDomain restricts the built set to tasks matching the given domain.
func (b *EvalSetBuilder) FilterByDomain(domain EvalDomain) *EvalSetBuilder {
	b.filterDomain = &domain
	return b
}

// FilterByTags restricts the built set to tasks that have any of the given tags.
func (b *EvalSetBuilder) FilterByTags(tags []string) *EvalSetBuilder {
	b.filterTags = tags
	return b
}

// Limit caps the total number of tasks in the resulting eval set.
func (b *EvalSetBuilder) Limit(n int) *EvalSetBuilder {
	b.limit = n
	return b
}

// Build validates inputs, applies filters and limits, and returns the EvalSet.
func (b *EvalSetBuilder) Build() (*EvalSet, error) {
	// Apply filters to produce the final task list.
	filtered := b.applyFilters(b.tasks)

	if len(filtered) == 0 {
		return nil, errors.New("eval set must contain at least one task")
	}

	// Apply limit.
	if b.limit > 0 && len(filtered) > b.limit {
		filtered = filtered[:b.limit]
	}

	// Normalise weights: default zero weights to 1.0.
	for i := range filtered {
		if filtered[i].Weight == 0 {
			filtered[i].Weight = 1.0
		}
	}

	set := &EvalSet{
		Config: EvalSetConfig{
			Version:          b.version,
			Name:             b.name,
			Description:      b.description,
			CompositionRules: b.rules,
		},
		Tasks:     filtered,
		CreatedAt: time.Now(),
	}

	return set, nil
}

// applyFilters returns a new slice containing only tasks that pass all active filters.
func (b *EvalSetBuilder) applyFilters(tasks []EvalTask) []EvalTask {
	result := make([]EvalTask, 0, len(tasks))
	for _, t := range tasks {
		if b.filterDifficulty != nil && t.Difficulty != *b.filterDifficulty {
			continue
		}
		if b.filterDomain != nil && t.Domain != *b.filterDomain {
			continue
		}
		if len(b.filterTags) > 0 && !hasAnyTag(t.Tags, b.filterTags) {
			continue
		}
		result = append(result, t)
	}
	return result
}

// hasAnyTag reports whether taskTags contains at least one of the target tags.
func hasAnyTag(taskTags, targetTags []string) bool {
	set := make(map[string]struct{}, len(taskTags))
	for _, tag := range taskTags {
		set[tag] = struct{}{}
	}
	for _, tag := range targetTags {
		if _, ok := set[tag]; ok {
			return true
		}
	}
	return false
}
