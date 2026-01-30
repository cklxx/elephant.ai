package skills

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// AutoActivationConfig controls automatic skill activation behavior.
type AutoActivationConfig struct {
	Enabled             bool
	MaxActivated        int
	TokenBudget         int
	ConfidenceThreshold float64
	FallbackToIndex     bool
}

// MatchContext supplies context for skill matching.
type MatchContext struct {
	TaskInput   string
	RecentTools []string
	Slots       map[string]string
	SessionID   string
}

// MatchSignal captures the signal contribution for a match.
type MatchSignal struct {
	Type   string
	Detail string
	Weight float64
}

// MatchResult represents a scored skill match.
type MatchResult struct {
	Skill   Skill
	Score   float64
	Signals []MatchSignal
}

// MatcherOptions customize skill matcher behavior.
type MatcherOptions struct {
	FeedbackStore *FeedbackStore
}

// SkillMatcher performs multi-signal matching for skills.
type SkillMatcher struct {
	library         *Library
	compiledRegex   map[string][]*regexp.Regexp
	cooldownTracker map[string]time.Time
	mu              sync.RWMutex
	feedbackStore   *FeedbackStore
}

// NewSkillMatcher constructs a matcher with precompiled regex patterns.
func NewSkillMatcher(library *Library, opts MatcherOptions) *SkillMatcher {
	m := &SkillMatcher{
		library:         library,
		compiledRegex:   make(map[string][]*regexp.Regexp),
		cooldownTracker: make(map[string]time.Time),
		feedbackStore:   opts.FeedbackStore,
	}
	if library == nil {
		return m
	}
	for _, skill := range library.List() {
		if skill.Triggers == nil || len(skill.Triggers.IntentPatterns) == 0 {
			continue
		}
		var compiled []*regexp.Regexp
		for _, pattern := range skill.Triggers.IntentPatterns {
			trimmed := strings.TrimSpace(pattern)
			if trimmed == "" {
				continue
			}
			if re, err := regexp.Compile("(?i)" + trimmed); err == nil {
				compiled = append(compiled, re)
			}
		}
		if len(compiled) > 0 {
			m.compiledRegex[skill.Name] = compiled
		}
	}
	return m
}

// Match returns the activated skills based on the supplied context.
func (m *SkillMatcher) Match(ctx MatchContext, cfg AutoActivationConfig) []MatchResult {
	if m == nil || m.library == nil || !cfg.Enabled {
		return nil
	}

	var candidates []MatchResult
	for _, skill := range m.library.List() {
		if skill.Triggers == nil {
			continue
		}
		result := m.scoreSkill(skill, ctx)
		threshold := skill.Triggers.ConfidenceThreshold
		if threshold == 0 {
			if cfg.ConfidenceThreshold > 0 {
				threshold = cfg.ConfidenceThreshold
			} else {
				threshold = 0.5
			}
		}
		if result.Score >= threshold {
			candidates = append(candidates, result)
		}
	}

	resolved := m.resolveConflicts(candidates, ctx.SessionID)
	sort.SliceStable(resolved, func(i, j int) bool {
		if resolved[i].Score == resolved[j].Score {
			return resolved[i].Skill.Priority > resolved[j].Skill.Priority
		}
		return resolved[i].Score > resolved[j].Score
	})
	if cfg.MaxActivated > 0 && len(resolved) > cfg.MaxActivated {
		resolved = resolved[:cfg.MaxActivated]
	}
	return resolved
}

// ApplyActivationLimits enforces token budget limits on matched skills.
func ApplyActivationLimits(matches []MatchResult, cfg AutoActivationConfig) []MatchResult {
	if len(matches) == 0 || cfg.TokenBudget <= 0 {
		return matches
	}

	used := 0
	var out []MatchResult
	for _, match := range matches {
		body := match.Skill.Body
		if len(match.Skill.Chain) > 0 {
			body = ""
		}
		if strings.TrimSpace(body) == "" && len(match.Skill.Chain) == 0 {
			continue
		}
		tokenCost := EstimateTokens(body)
		if tokenCost == 0 && len(match.Skill.Chain) > 0 {
			tokenCost = match.Skill.MaxTokens
		}
		maxTokens := match.Skill.MaxTokens
		if maxTokens > 0 && tokenCost > maxTokens {
			tokenCost = maxTokens
		}
		if used+tokenCost > cfg.TokenBudget {
			break
		}
		used += tokenCost
		out = append(out, match)
	}
	return out
}

// MarkActivated records activation timestamps for cooldown enforcement.
func (m *SkillMatcher) MarkActivated(sessionID string, matches []MatchResult) {
	if m == nil || len(matches) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, match := range matches {
		if match.Skill.Cooldown <= 0 {
			continue
		}
		key := cooldownKey(sessionID, match.Skill.Name)
		m.cooldownTracker[key] = time.Now()
	}
}

func (m *SkillMatcher) scoreSkill(skill Skill, ctx MatchContext) MatchResult {
	result := MatchResult{Skill: skill}
	var totalWeight float64

	input := strings.ToLower(ctx.TaskInput)

	// Stage 1: intent patterns (0.6)
	if regexes, ok := m.compiledRegex[skill.Name]; ok {
		for _, re := range regexes {
			if re.MatchString(ctx.TaskInput) {
				result.Signals = append(result.Signals, MatchSignal{
					Type:   "intent_pattern",
					Detail: re.String(),
					Weight: 0.6,
				})
				totalWeight += 0.6
				break
			}
		}
	}

	// Stage 2a: tool signals (0.25)
	if skill.Triggers.ToolSignals != nil && len(ctx.RecentTools) > 0 {
		for _, signal := range skill.Triggers.ToolSignals {
			for _, tool := range ctx.RecentTools {
				if strings.EqualFold(tool, signal) {
					result.Signals = append(result.Signals, MatchSignal{
						Type:   "tool_signal",
						Detail: signal,
						Weight: 0.25,
					})
					totalWeight += 0.25
					break
				}
			}
		}
	}

	// Stage 2b: context keyword matching (0.15)
	if cs := skill.Triggers.ContextSignals; cs != nil {
		matchedKeywords := 0
		for _, kw := range cs.Keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(input, strings.ToLower(kw)) {
				matchedKeywords++
			}
		}
		if len(cs.Keywords) > 0 && matchedKeywords > 0 {
			ratio := float64(matchedKeywords) / float64(len(cs.Keywords))
			weight := 0.15 * ratio
			result.Signals = append(result.Signals, MatchSignal{
				Type:   "context_keyword",
				Detail: fmt.Sprintf("%d/%d keywords", matchedKeywords, len(cs.Keywords)),
				Weight: weight,
			})
			totalWeight += weight
		}

		// Stage 2c: slot matching (0.1)
		if len(cs.Slots) > 0 && len(ctx.Slots) > 0 {
			for slotKey, slotValues := range cs.Slots {
				if ctxValue, ok := ctx.Slots[slotKey]; ok {
					for _, sv := range slotValues {
						if ctxValue == sv {
							result.Signals = append(result.Signals, MatchSignal{
								Type:   "context_slot",
								Detail: fmt.Sprintf("%s=%s", slotKey, sv),
								Weight: 0.1,
							})
							totalWeight += 0.1
							break
						}
					}
				}
			}
		}
	}

	if totalWeight > 1.0 {
		totalWeight = 1.0
	}

	result.Score = m.adjustScoreForFeedback(skill, totalWeight)
	return result
}

func (m *SkillMatcher) adjustScoreForFeedback(skill Skill, base float64) float64 {
	if m.feedbackStore == nil {
		return base
	}
	stats, ok := m.feedbackStore.GetStats(skill.Name)
	if !ok || (stats.Helpful+stats.NotHelpful) == 0 {
		return base
	}
	ratio := stats.HelpfulRatio()
	if ratio <= 0 {
		return base * 0.7
	}
	return base * (0.8 + 0.4*ratio)
}

func (m *SkillMatcher) resolveConflicts(candidates []MatchResult, sessionID string) []MatchResult {
	if len(candidates) == 0 {
		return nil
	}

	filtered := make([]MatchResult, 0, len(candidates))
	now := time.Now()
	m.mu.RLock()
	for _, c := range candidates {
		if c.Skill.Cooldown <= 0 {
			filtered = append(filtered, c)
			continue
		}
		last, ok := m.cooldownTracker[cooldownKey(sessionID, c.Skill.Name)]
		if ok {
			cooldown := time.Duration(c.Skill.Cooldown) * time.Second
			if cooldown > 0 && now.Sub(last) < cooldown {
				continue
			}
		}
		filtered = append(filtered, c)
	}
	m.mu.RUnlock()

	groupWinners := make(map[string]MatchResult)
	var noGroup []MatchResult
	for _, c := range filtered {
		if strings.TrimSpace(c.Skill.ExclusiveGroup) == "" {
			noGroup = append(noGroup, c)
			continue
		}
		existing, ok := groupWinners[c.Skill.ExclusiveGroup]
		if !ok || c.Skill.Priority > existing.Skill.Priority || (c.Skill.Priority == existing.Skill.Priority && c.Score > existing.Score) {
			groupWinners[c.Skill.ExclusiveGroup] = c
		}
	}

	result := append([]MatchResult{}, noGroup...)
	for _, winner := range groupWinners {
		result = append(result, winner)
	}
	return result
}

func cooldownKey(sessionID, skillName string) string {
	if sessionID == "" {
		return NormalizeName(skillName)
	}
	return sessionID + ":" + NormalizeName(skillName)
}
