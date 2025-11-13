package app

import (
	"context"

	"alex/internal/agent/ports"
	"alex/internal/rag/gate"
)

// RAGGateAdapter bridges the internal gate implementation with the agent ports interface.
type RAGGateAdapter struct {
	gate gate.Gate
}

// NewRAGGateAdapter constructs an adapter around the provided gate.
func NewRAGGateAdapter(g gate.Gate) *RAGGateAdapter {
	return &RAGGateAdapter{gate: g}
}

// Evaluate delegates to the underlying gate and converts results into the ports representation.
func (a *RAGGateAdapter) Evaluate(ctx context.Context, signals ports.RAGSignals) ports.RAGDirectives {
	decision := a.gate.Evaluate(ctx, gate.Signals{
		Query:             signals.Query,
		RetrievalHitRate:  signals.RetrievalHitRate,
		FreshnessGapHours: signals.FreshnessGapHours,
		IntentConfidence:  signals.IntentConfidence,
		BudgetRemaining:   signals.BudgetRemaining,
		BudgetTarget:      signals.BudgetTarget,
		CanRetrieve:       signals.CanRetrieve,
		AllowSearch:       signals.AllowSearch,
		AllowCrawl:        signals.AllowCrawl,
		SearchSeeds:       append([]string(nil), signals.SearchSeeds...),
		CrawlSeeds:        append([]string(nil), signals.CrawlSeeds...),
	})

	return ports.RAGDirectives{
		Query:         decision.Query,
		UseRetrieval:  decision.UseRetrieval,
		UseSearch:     decision.UseSearch,
		UseCrawl:      decision.UseCrawl,
		SearchSeeds:   append([]string(nil), decision.SearchSeeds...),
		CrawlSeeds:    append([]string(nil), decision.CrawlSeeds...),
		Justification: copyJustification(decision.Justification),
	}
}

func copyJustification(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
