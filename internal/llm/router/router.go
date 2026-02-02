// Package router provides dynamic model selection for LLM requests.
//
// It scores available model profiles against a RoutingRequest using
// configurable cost, latency, and capability weights, filters out
// unhealthy providers, and returns a ranked RoutingResult with fallbacks.
package router

import (
	"context"
	"math"
	"sort"
	"sync"
)

// ---------------------------------------------------------------------------
// Model tier
// ---------------------------------------------------------------------------

// ModelTier classifies a model by capability/cost tradeoff.
type ModelTier string

const (
	TierSmall   ModelTier = "small"   // fast/cheap model for simple tasks
	TierDefault ModelTier = "default" // standard model for most tasks
	TierStrong  ModelTier = "strong"  // most capable model for complex tasks
)

// tierRank returns a numeric rank for ordering tiers (higher = stronger).
func tierRank(t ModelTier) int {
	switch t {
	case TierSmall:
		return 0
	case TierDefault:
		return 1
	case TierStrong:
		return 2
	default:
		return 1
	}
}

// ---------------------------------------------------------------------------
// Model profile
// ---------------------------------------------------------------------------

// ModelProfile describes a single model available for routing.
type ModelProfile struct {
	Provider         string   // provider name (openai, anthropic, etc.)
	Model            string   // model identifier
	Tier             ModelTier
	MaxContextTokens int      // max context window
	CostPer1KInput  float64  // cost in USD per 1K input tokens
	CostPer1KOutput float64  // cost in USD per 1K output tokens
	AvgLatencyMs     float64  // expected average latency in milliseconds
	Capabilities     []string // "code", "reasoning", "vision", etc.
}

// hasCapability reports whether the profile advertises the given capability.
func (p ModelProfile) hasCapability(cap string) bool {
	for _, c := range p.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// hasAllCapabilities reports whether the profile advertises every capability
// in the given list.
func (p ModelProfile) hasAllCapabilities(caps []string) bool {
	for _, cap := range caps {
		if !p.hasCapability(cap) {
			return false
		}
	}
	return true
}

// estimatedRequestCost returns the approximate cost of a single request
// with the given number of input tokens (assuming output is roughly 1/4 of input).
func (p ModelProfile) estimatedRequestCost(inputTokens int) float64 {
	outputEstimate := float64(inputTokens) / 4.0
	return (float64(inputTokens)/1000.0)*p.CostPer1KInput +
		(outputEstimate/1000.0)*p.CostPer1KOutput
}

// ---------------------------------------------------------------------------
// Routing request / result
// ---------------------------------------------------------------------------

// RoutingRequest carries all signals the router uses to select a model.
type RoutingRequest struct {
	TaskComplexity       string    // "simple", "complex", "" (from pre-analysis)
	EstimatedTokens      int       // estimated context size
	RequiredCapabilities []string  // capabilities needed
	PreferredTier        ModelTier // hint from pre-analysis
	MaxCostPerRequest    float64   // cost budget per request (0 = no limit)
	SessionID            string
	UserID               string
}

// RoutingResult is the output of a routing decision.
type RoutingResult struct {
	Profile   ModelProfile   // selected model
	Reason    string         // why this model was chosen
	Fallbacks []ModelProfile // alternative models if primary fails
}

// ---------------------------------------------------------------------------
// Router config
// ---------------------------------------------------------------------------

// RouterConfig configures the Router.
type RouterConfig struct {
	Models        []ModelProfile // available model profiles
	DefaultTier   ModelTier      // default when no signal
	CostWeight    float64        // 0-1, how much to weight cost vs capability (default 0.3)
	LatencyWeight float64        // 0-1, how much to weight latency (default 0.2)
}

// defaults fills zero-valued config fields with sensible defaults.
func (c *RouterConfig) defaults() {
	if c.DefaultTier == "" {
		c.DefaultTier = TierDefault
	}
	if c.CostWeight == 0 && c.LatencyWeight == 0 {
		c.CostWeight = 0.3
		c.LatencyWeight = 0.2
	}
}

// ---------------------------------------------------------------------------
// Router
// ---------------------------------------------------------------------------

// Router selects the best model profile for a given request.
type Router struct {
	mu             sync.RWMutex
	models         []ModelProfile
	defaultTier    ModelTier
	costWeight     float64
	latencyWeight  float64
	providerHealth map[string]bool // provider -> healthy; absent means healthy
}

// NewRouter creates a Router from the given configuration.
func NewRouter(cfg RouterConfig) *Router {
	cfg.defaults()
	models := make([]ModelProfile, len(cfg.Models))
	copy(models, cfg.Models)
	return &Router{
		models:         models,
		defaultTier:    cfg.DefaultTier,
		costWeight:     cfg.CostWeight,
		latencyWeight:  cfg.LatencyWeight,
		providerHealth: make(map[string]bool),
	}
}

// RegisterModel adds or updates a model profile. If a profile with the same
// Provider+Model already exists it is replaced; otherwise it is appended.
func (r *Router) RegisterModel(profile ModelProfile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, m := range r.models {
		if m.Provider == profile.Provider && m.Model == profile.Model {
			r.models[i] = profile
			return
		}
	}
	r.models = append(r.models, profile)
}

// AvailableModels returns all registered models for the given tier.
func (r *Router) AvailableModels(tier ModelTier) []ModelProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ModelProfile
	for _, m := range r.models {
		if m.Tier == tier {
			out = append(out, m)
		}
	}
	return out
}

// SetProviderHealth marks a provider as healthy or unhealthy.
func (r *Router) SetProviderHealth(provider string, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providerHealth[provider] = healthy
}

// isProviderHealthy returns true if the provider has not been marked unhealthy.
// A provider not present in the map is assumed healthy.
func (r *Router) isProviderHealthy(provider string) bool {
	healthy, ok := r.providerHealth[provider]
	if !ok {
		return true
	}
	return healthy
}

// Route selects the best model for the given request.
func (r *Router) Route(_ context.Context, req RoutingRequest) RoutingResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// --- Step 1: filter candidates ---
	candidates := r.filterCandidates(req)

	// --- Step 2: if nothing matched, fall back to default tier ---
	if len(candidates) == 0 {
		return r.defaultFallback()
	}

	// --- Step 3: determine the target tier ---
	targetTier := r.resolveTargetTier(req)

	// --- Step 4: prefer models in the target tier ---
	tierMatches := filterByTier(candidates, targetTier)

	primary := candidates
	if len(tierMatches) > 0 {
		primary = tierMatches
	}

	// --- Step 5: score and rank ---
	best := r.scoreAndSelect(primary)

	// --- Step 6: build fallbacks ---
	fallbacks := r.buildFallbacks(candidates, best, targetTier)

	reason := "scored_best"
	if len(tierMatches) > 0 {
		reason = "tier_match_" + string(targetTier)
	}

	return RoutingResult{
		Profile:   best,
		Reason:    reason,
		Fallbacks: fallbacks,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// filterCandidates applies hard filters: capabilities, context window, health, cost budget.
func (r *Router) filterCandidates(req RoutingRequest) []ModelProfile {
	var out []ModelProfile
	for _, m := range r.models {
		// capability filter
		if len(req.RequiredCapabilities) > 0 && !m.hasAllCapabilities(req.RequiredCapabilities) {
			continue
		}
		// context window filter
		if req.EstimatedTokens > 0 && m.MaxContextTokens > 0 && req.EstimatedTokens > m.MaxContextTokens {
			continue
		}
		// health filter
		if !r.isProviderHealthy(m.Provider) {
			continue
		}
		// cost budget filter
		if req.MaxCostPerRequest > 0 && req.EstimatedTokens > 0 {
			est := m.estimatedRequestCost(req.EstimatedTokens)
			if est > req.MaxCostPerRequest {
				continue
			}
		}
		out = append(out, m)
	}
	return out
}

// resolveTargetTier determines the tier the router should prefer.
func (r *Router) resolveTargetTier(req RoutingRequest) ModelTier {
	// PreferredTier overrides complexity-based heuristic.
	if req.PreferredTier != "" {
		return req.PreferredTier
	}
	switch req.TaskComplexity {
	case "simple":
		return TierSmall
	case "complex":
		return TierStrong
	default:
		return r.defaultTier
	}
}

// filterByTier returns only the models matching the given tier.
func filterByTier(models []ModelProfile, tier ModelTier) []ModelProfile {
	var out []ModelProfile
	for _, m := range models {
		if m.Tier == tier {
			out = append(out, m)
		}
	}
	return out
}

// scoreAndSelect scores a non-empty list of candidates and returns the best one.
func (r *Router) scoreAndSelect(candidates []ModelProfile) ModelProfile {
	if len(candidates) == 1 {
		return candidates[0]
	}

	// Find min/max cost and latency for normalization.
	minCost, maxCost := math.MaxFloat64, -math.MaxFloat64
	minLat, maxLat := math.MaxFloat64, -math.MaxFloat64
	for _, m := range candidates {
		cost := m.CostPer1KInput + m.CostPer1KOutput
		if cost < minCost {
			minCost = cost
		}
		if cost > maxCost {
			maxCost = cost
		}
		lat := m.AvgLatencyMs
		if lat < minLat {
			minLat = lat
		}
		if lat > maxLat {
			maxLat = lat
		}
	}

	capWeight := 1.0 - r.costWeight - r.latencyWeight
	if capWeight < 0 {
		capWeight = 0
	}

	bestScore := -math.MaxFloat64
	bestIdx := 0
	for i, m := range candidates {
		score := r.score(m, capWeight, minCost, maxCost, minLat, maxLat)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return candidates[bestIdx]
}

// score computes a composite score for a single model.
func (r *Router) score(m ModelProfile, capWeight, minCost, maxCost, minLat, maxLat float64) float64 {
	// Capability score: use tier rank as proxy (higher = better).
	capScore := float64(tierRank(m.Tier)) / 2.0 // normalize to 0-1

	// Cost score: lower is better → invert.
	costScore := 1.0
	costRange := maxCost - minCost
	if costRange > 0 {
		cost := m.CostPer1KInput + m.CostPer1KOutput
		costScore = 1.0 - (cost-minCost)/costRange
	}

	// Latency score: lower is better → invert.
	latScore := 1.0
	latRange := maxLat - minLat
	if latRange > 0 {
		latScore = 1.0 - (m.AvgLatencyMs-minLat)/latRange
	}

	return capWeight*capScore + r.costWeight*costScore + r.latencyWeight*latScore
}

// defaultFallback returns a RoutingResult using the first model of the
// default tier, or the very first model if no default-tier model exists.
func (r *Router) defaultFallback() RoutingResult {
	for _, m := range r.models {
		if m.Tier == r.defaultTier {
			return RoutingResult{
				Profile: m,
				Reason:  "no_match_fallback_to_default",
			}
		}
	}
	if len(r.models) > 0 {
		return RoutingResult{
			Profile: r.models[0],
			Reason:  "no_match_fallback_to_default",
		}
	}
	return RoutingResult{
		Reason: "no_models_available",
	}
}

// buildFallbacks assembles a fallback list from the candidates, excluding the
// primary selection, preferring the next tier up (if primary is small) or down
// (if primary is strong).
func (r *Router) buildFallbacks(candidates []ModelProfile, primary ModelProfile, targetTier ModelTier) []ModelProfile {
	// Determine fallback tier order.
	var preferredFallbackTiers []ModelTier
	switch targetTier {
	case TierSmall:
		preferredFallbackTiers = []ModelTier{TierDefault, TierStrong}
	case TierStrong:
		preferredFallbackTiers = []ModelTier{TierDefault, TierSmall}
	default:
		preferredFallbackTiers = []ModelTier{TierStrong, TierSmall}
	}

	type scored struct {
		profile  ModelProfile
		tierPrio int // lower = preferred
	}

	var items []scored
	for _, m := range candidates {
		if m.Provider == primary.Provider && m.Model == primary.Model {
			continue
		}
		prio := len(preferredFallbackTiers) // default: lowest priority
		for i, ft := range preferredFallbackTiers {
			if m.Tier == ft {
				prio = i
				break
			}
		}
		items = append(items, scored{profile: m, tierPrio: prio})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].tierPrio != items[j].tierPrio {
			return items[i].tierPrio < items[j].tierPrio
		}
		// Within the same tier priority, prefer lower cost.
		ci := items[i].profile.CostPer1KInput + items[i].profile.CostPer1KOutput
		cj := items[j].profile.CostPer1KInput + items[j].profile.CostPer1KOutput
		return ci < cj
	})

	fallbacks := make([]ModelProfile, 0, len(items))
	for _, item := range items {
		fallbacks = append(fallbacks, item.profile)
	}
	return fallbacks
}
