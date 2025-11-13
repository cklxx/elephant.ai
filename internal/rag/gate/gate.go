package gate

import (
	"context"
	"math"
)

// Decision captures the gate output indicating which retrieval actions to run.
type Decision struct {
	Query         string
	UseRetrieval  bool
	UseSearch     bool
	UseCrawl      bool
	SearchSeeds   []string
	CrawlSeeds    []string
	Justification map[string]float64
}

// Signals are the measurable inputs used to evaluate whether search or crawling
// should run for a given interaction.
type Signals struct {
	Query             string
	RetrievalHitRate  float64
	FreshnessGapHours float64
	IntentConfidence  float64
	BudgetRemaining   float64
	BudgetTarget      float64
	CanRetrieve       bool
	AllowSearch       bool
	AllowCrawl        bool
	SearchSeeds       []string
	CrawlSeeds        []string
}

// TelemetryEmitter is the minimal interface used to record gate decisions.
type TelemetryEmitter interface {
	EmitGateDecision(ctx context.Context, decision Decision, signals Signals)
}

// NopEmitter can be used when no telemetry integration is required.
type NopEmitter struct{}

// EmitGateDecision implements TelemetryEmitter.
func (NopEmitter) EmitGateDecision(context.Context, Decision, Signals) {}

// Config captures the tunable thresholds used by the gate.
type Config struct {
	RetrieveSatisfiedThreshold float64
	SearchTriggerThreshold     float64
	FullLoopTriggerThreshold   float64
	FreshnessHalfLifeHours     float64
	CoverageWeight             float64
	FreshnessWeight            float64
	IntentWeight               float64
	CostWeight                 float64
	MaxSearchSeeds             int
	MaxCrawlSeeds              int
}

// DefaultConfig returns a Config tuned to prefer local retrieval while still
// allowing discovery when evidence suggests it is necessary.
func DefaultConfig() Config {
	return Config{
		RetrieveSatisfiedThreshold: 0.7,
		SearchTriggerThreshold:     0.45,
		FullLoopTriggerThreshold:   0.75,
		FreshnessHalfLifeHours:     72,
		CoverageWeight:             0.4,
		FreshnessWeight:            0.3,
		IntentWeight:               0.2,
		CostWeight:                 0.1,
		MaxSearchSeeds:             5,
		MaxCrawlSeeds:              5,
	}
}

// Gate evaluates Signals to determine which retrieval directives to execute.
type Gate struct {
	cfg     Config
	emitter TelemetryEmitter
}

// New creates a Gate using the provided Config and TelemetryEmitter.
func New(cfg Config, emitter TelemetryEmitter) Gate {
	defaults := DefaultConfig()
	if cfg.RetrieveSatisfiedThreshold <= 0 || cfg.RetrieveSatisfiedThreshold > 1 {
		cfg.RetrieveSatisfiedThreshold = defaults.RetrieveSatisfiedThreshold
	}
	if cfg.SearchTriggerThreshold <= 0 {
		cfg.SearchTriggerThreshold = defaults.SearchTriggerThreshold
	}
	if cfg.FullLoopTriggerThreshold <= 0 {
		cfg.FullLoopTriggerThreshold = defaults.FullLoopTriggerThreshold
	}
	if cfg.FreshnessHalfLifeHours <= 0 {
		cfg.FreshnessHalfLifeHours = defaults.FreshnessHalfLifeHours
	}
	if cfg.CoverageWeight < 0 {
		cfg.CoverageWeight = defaults.CoverageWeight
	}
	if cfg.FreshnessWeight < 0 {
		cfg.FreshnessWeight = defaults.FreshnessWeight
	}
	if cfg.IntentWeight < 0 {
		cfg.IntentWeight = defaults.IntentWeight
	}
	if cfg.CostWeight < 0 {
		cfg.CostWeight = defaults.CostWeight
	}
	if cfg.MaxSearchSeeds <= 0 {
		cfg.MaxSearchSeeds = defaults.MaxSearchSeeds
	}
	if cfg.MaxCrawlSeeds <= 0 {
		cfg.MaxCrawlSeeds = defaults.MaxCrawlSeeds
	}
	if emitter == nil {
		emitter = NopEmitter{}
	}
	return Gate{cfg: cfg, emitter: emitter}
}

// Evaluate calculates the retrieval directives from the provided Signals.
func (g Gate) Evaluate(ctx context.Context, s Signals) Decision {
	decision := Decision{
		Query:         s.Query,
		SearchSeeds:   truncateStrings(s.SearchSeeds, g.cfg.MaxSearchSeeds),
		CrawlSeeds:    truncateStrings(s.CrawlSeeds, g.cfg.MaxCrawlSeeds),
		Justification: make(map[string]float64),
	}

	if !s.CanRetrieve {
		decision.Justification["can_retrieve"] = 0
		g.emitter.EmitGateDecision(ctx, decision, s)
		return decision
	}

	decision.UseRetrieval = true

	coverageShortfall := clamp01(1 - s.RetrievalHitRate)
	freshnessScore := clamp01(s.FreshnessGapHours / g.cfg.FreshnessHalfLifeHours)
	intentScore := clamp01(s.IntentConfidence)
	costPressure := 0.0
	if s.BudgetTarget > 0 {
		normalizedBudget := s.BudgetRemaining / s.BudgetTarget
		costPressure = clamp01(1 - normalizedBudget)
	}

	score := g.cfg.CoverageWeight*coverageShortfall +
		g.cfg.FreshnessWeight*freshnessScore +
		g.cfg.IntentWeight*intentScore +
		g.cfg.CostWeight*costPressure

	decision.Justification["coverage_shortfall"] = coverageShortfall
	decision.Justification["freshness_score"] = freshnessScore
	decision.Justification["intent_score"] = intentScore
	decision.Justification["cost_pressure"] = costPressure
	decision.Justification["total_score"] = score
	decision.Justification["retrieval_hit_rate"] = clamp01(s.RetrievalHitRate)

	fullLoopEligible := score >= g.cfg.FullLoopTriggerThreshold
	searchEligible := score >= g.cfg.SearchTriggerThreshold

	if fullLoopEligible {
		if s.AllowSearch {
			decision.UseSearch = true
		} else {
			decision.Justification["search_blocked"] = 1
		}
		if s.AllowCrawl {
			decision.UseCrawl = true
		} else {
			decision.Justification["crawl_blocked"] = 1
		}
	} else if searchEligible {
		if s.AllowSearch {
			decision.UseSearch = true
		} else {
			decision.Justification["search_blocked"] = 1
		}
	} else if s.RetrievalHitRate < g.cfg.RetrieveSatisfiedThreshold {
		if !s.AllowSearch && !s.AllowCrawl {
			decision.Justification["policy_block"] = 1
		}
	}

	if decision.UseCrawl && !decision.UseSearch {
		// Crawling depends on search to seed discovery. Downgrade gracefully.
		decision.UseCrawl = false
		decision.Justification["crawl_blocked"] = 1
	}

	if (!decision.UseSearch && searchEligible) || (!decision.UseCrawl && fullLoopEligible) {
		decision.Justification["policy_block"] = 1
	}

	g.emitter.EmitGateDecision(ctx, decision, s)
	return decision
}

func truncateStrings(values []string, limit int) []string {
	if len(values) == 0 || limit <= 0 {
		return nil
	}
	if len(values) <= limit {
		trimmed := make([]string, len(values))
		copy(trimmed, values)
		return trimmed
	}
	trimmed := make([]string, limit)
	copy(trimmed, values[:limit])
	return trimmed
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
