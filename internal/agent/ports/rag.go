package ports

import "context"

// RAGDirectives capture which retrieval actions should run before execution.
type RAGDirectives struct {
	Query         string
	UseRetrieval  bool
	UseSearch     bool
	UseCrawl      bool
	SearchSeeds   []string
	CrawlSeeds    []string
	Justification map[string]float64
}

// RAGSignals enumerates the inputs used by the gate to score a request.
type RAGSignals struct {
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

// RAGGate evaluates RAGSignals and returns the directives that should be executed.
type RAGGate interface {
	Evaluate(ctx context.Context, signals RAGSignals) RAGDirectives
}
