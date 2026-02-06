package tools

import (
	"math"
	"sort"
	"sync"
)

// ToolSLAProfile is a computed view of a tool's current SLA metrics combined
// with a health score and routing recommendation.
type ToolSLAProfile struct {
	ToolName    string
	SLA         ToolSLA
	HealthScore float64
	Recommended bool
}

// SLARouterConfig holds the tunable thresholds and weights used by SLARouter
// to compute health scores and make routing decisions.
type SLARouterConfig struct {
	// MaxP95LatencyMs is the maximum acceptable P95 latency in milliseconds.
	// Tools exceeding this threshold receive a zero latency score.
	MaxP95LatencyMs float64

	// MaxErrorRate is the maximum acceptable error rate (0–1). Tools at or
	// above this threshold receive a zero error score.
	MaxErrorRate float64

	// MinSuccessRate is the minimum acceptable success rate (0–1). Tools
	// meeting or exceeding this threshold receive a full reliability score.
	MinSuccessRate float64

	// MinCallCount is the minimum number of recorded calls before the SLA
	// data is considered reliable. Tools below this threshold are assumed
	// healthy (score 1.0).
	MinCallCount int64

	// LatencyWeight is the weight applied to the latency component when
	// computing the composite health score.
	LatencyWeight float64

	// ErrorWeight is the weight applied to the error-rate component when
	// computing the composite health score.
	ErrorWeight float64

	// ReliabilityWeight is the weight applied to the success-rate component
	// when computing the composite health score.
	ReliabilityWeight float64
}

// DefaultSLARouterConfig returns an SLARouterConfig with sensible defaults.
func DefaultSLARouterConfig() SLARouterConfig {
	return SLARouterConfig{
		MaxP95LatencyMs:   5000,
		MaxErrorRate:      0.3,
		MinSuccessRate:    0.7,
		MinCallCount:      10,
		LatencyWeight:     0.4,
		ErrorWeight:       0.4,
		ReliabilityWeight: 0.2,
	}
}

// SLARouter uses live SLA metrics from an SLACollector to compute health
// scores, rank tools, and select the best alternative from a set of
// candidates. All methods are safe for concurrent use.
type SLARouter struct {
	collector *SLACollector
	config    SLARouterConfig
	mu        sync.RWMutex
}

// NewSLARouter creates a new SLARouter backed by the given SLACollector and
// configuration.
func NewSLARouter(collector *SLACollector, cfg SLARouterConfig) *SLARouter {
	return &SLARouter{
		collector: collector,
		config:    cfg,
	}
}

// ComputeHealthScore calculates a composite health score in [0, 1] from the
// given SLA snapshot.
//
// When CallCount < MinCallCount the tool lacks sufficient data and is assumed
// healthy (returns 1.0).
//
// Otherwise:
//
//	latencyScore     = 1.0 - min(P95ms / MaxP95LatencyMs, 1.0)
//	errorScore       = 1.0 - min(ErrorRate / MaxErrorRate, 1.0)
//	reliabilityScore = min(SuccessRate / MinSuccessRate, 1.0)
//	healthScore      = LatencyWeight*latencyScore + ErrorWeight*errorScore + ReliabilityWeight*reliabilityScore
//
// The result is clamped to [0, 1].
func (r *SLARouter) ComputeHealthScore(sla ToolSLA) float64 {
	r.mu.RLock()
	cfg := r.config
	r.mu.RUnlock()

	if sla.CallCount < cfg.MinCallCount {
		return 1.0
	}

	p95Ms := float64(sla.P95Latency.Milliseconds())
	latencyScore := 1.0 - math.Min(p95Ms/cfg.MaxP95LatencyMs, 1.0)
	errorScore := 1.0 - math.Min(sla.ErrorRate/cfg.MaxErrorRate, 1.0)
	reliabilityScore := math.Min(sla.SuccessRate/cfg.MinSuccessRate, 1.0)

	health := cfg.LatencyWeight*latencyScore +
		cfg.ErrorWeight*errorScore +
		cfg.ReliabilityWeight*reliabilityScore

	return math.Max(0.0, math.Min(1.0, health))
}

// GetProfile returns a ToolSLAProfile for the named tool with the current SLA
// metrics and computed health score. Recommended is true when the health score
// equals 1.0 or the tool has insufficient data (assumed healthy).
func (r *SLARouter) GetProfile(toolName string) ToolSLAProfile {
	sla := r.collector.GetSLA(toolName)
	score := r.ComputeHealthScore(sla)
	return ToolSLAProfile{
		ToolName:    toolName,
		SLA:         sla,
		HealthScore: score,
		Recommended: score >= 0.7,
	}
}

// RankTools returns profiles for the given tool names sorted by health score
// in descending order (best first). Tools with equal scores retain their
// original order.
func (r *SLARouter) RankTools(toolNames []string) []ToolSLAProfile {
	profiles := make([]ToolSLAProfile, len(toolNames))
	for i, name := range toolNames {
		profiles[i] = r.GetProfile(name)
	}
	sort.SliceStable(profiles, func(i, j int) bool {
		return profiles[i].HealthScore > profiles[j].HealthScore
	})
	return profiles
}

// SelectBest returns the tool name with the highest health score from the
// given candidates. If toolNames is empty it returns ("", false).
func (r *SLARouter) SelectBest(toolNames []string) (string, bool) {
	if len(toolNames) == 0 {
		return "", false
	}
	ranked := r.RankTools(toolNames)
	return ranked[0].ToolName, true
}

// IsHealthy returns true when the named tool's health score is at or above
// 0.7 (the recommendation threshold).
func (r *SLARouter) IsHealthy(toolName string) bool {
	profile := r.GetProfile(toolName)
	return profile.HealthScore >= 0.7
}
