package gate

import (
	"context"
	"testing"
)

type recordingEmitter struct {
	decision Decision
	signals  Signals
}

func (r *recordingEmitter) EmitGateDecision(_ context.Context, decision Decision, signals Signals) {
	r.decision = decision
	r.signals = signals
}

func TestGateEvaluatePlans(t *testing.T) {
	ctx := context.Background()
	emitter := &recordingEmitter{}
	gate := New(DefaultConfig(), emitter)

	t.Run("falls back to retrieval when coverage is strong", func(t *testing.T) {
		signals := Signals{
			Query:             "explain module wiring",
			RetrievalHitRate:  0.9,
			FreshnessGapHours: 12,
			IntentConfidence:  0.2,
			BudgetRemaining:   15,
			BudgetTarget:      20,
			CanRetrieve:       true,
			AllowSearch:       true,
			AllowCrawl:        true,
		}

		decision := gate.Evaluate(ctx, signals)
		if !decision.UseRetrieval || decision.UseSearch || decision.UseCrawl {
			t.Fatalf("expected retrieval only, got %+v", decision)
		}
		if decision.Justification["total_score"] >= gate.cfg.SearchTriggerThreshold {
			t.Fatalf("expected score below search threshold, got %f", decision.Justification["total_score"])
		}
		if !emitter.decision.UseRetrieval || emitter.decision.UseSearch {
			t.Fatalf("emitter saw unexpected directives %+v", emitter.decision)
		}
	})

	t.Run("promotes to search when coverage drops", func(t *testing.T) {
		signals := Signals{
			Query:             "latest golang security patches",
			RetrievalHitRate:  0.2,
			FreshnessGapHours: 60,
			IntentConfidence:  0.7,
			BudgetRemaining:   5,
			BudgetTarget:      20,
			CanRetrieve:       true,
			AllowSearch:       true,
			AllowCrawl:        false,
		}

		decision := gate.Evaluate(ctx, signals)
		if !decision.UseRetrieval || !decision.UseSearch || decision.UseCrawl {
			t.Fatalf("expected retrieval+search, got %+v", decision)
		}
		if _, blocked := decision.Justification["crawl_blocked"]; !blocked {
			t.Fatalf("expected crawl_blocked flag when crawl suppressed by policy")
		}
	})

	t.Run("escalates to full loop when search and crawl allowed", func(t *testing.T) {
		signals := Signals{
			Query:             "marketing insights",
			RetrievalHitRate:  0.1,
			FreshnessGapHours: 200,
			IntentConfidence:  0.9,
			BudgetRemaining:   8,
			BudgetTarget:      10,
			CanRetrieve:       true,
			AllowSearch:       true,
			AllowCrawl:        true,
			SearchSeeds:       []string{"example.com", "docs.example"},
			CrawlSeeds:        []string{"https://example.com/blog"},
		}

		decision := gate.Evaluate(ctx, signals)
		if !decision.UseRetrieval || !decision.UseSearch || !decision.UseCrawl {
			t.Fatalf("expected retrieval+search+crawl got %+v", decision)
		}
		if len(decision.SearchSeeds) == 0 || len(decision.CrawlSeeds) == 0 {
			t.Fatalf("expected seeds to be propagated in directives: %#v", decision)
		}
	})

	t.Run("handles policy restrictions by downgrading plan", func(t *testing.T) {
		signals := Signals{
			Query:             "public benchmarks",
			RetrievalHitRate:  0.05,
			FreshnessGapHours: 150,
			IntentConfidence:  0.8,
			BudgetRemaining:   4,
			BudgetTarget:      10,
			CanRetrieve:       true,
			AllowSearch:       false,
			AllowCrawl:        false,
		}

		decision := gate.Evaluate(ctx, signals)
		if !decision.UseRetrieval || decision.UseSearch || decision.UseCrawl {
			t.Fatalf("expected retrieval only after downgrade got %+v", decision)
		}
		if _, ok := decision.Justification["policy_block"]; !ok {
			t.Fatalf("expected policy_block justification when search/crawl disabled")
		}
	})
}

func TestTruncateStrings(t *testing.T) {
	input := []string{"a", "b", "c", "d"}

	trimmed := truncateStrings(input, 2)
	if len(trimmed) != 2 {
		t.Fatalf("expected 2 elements got %d", len(trimmed))
	}
	if trimmed[0] != "a" || trimmed[1] != "b" {
		t.Fatalf("unexpected values: %#v", trimmed)
	}

	trimmed = truncateStrings(input, 10)
	if len(trimmed) != len(input) {
		t.Fatalf("expected same length when limit large enough")
	}
	if &trimmed[0] == &input[0] {
		t.Fatalf("expected copy of slice not shared backing array")
	}

	if result := truncateStrings(nil, 2); result != nil {
		t.Fatalf("expected nil when input empty, got %#v", result)
	}
}

func TestClamp01(t *testing.T) {
	cases := []struct {
		value    float64
		expected float64
	}{
		{-0.5, 0},
		{0, 0},
		{0.25, 0.25},
		{1, 1},
		{1.2, 1},
	}

	for _, tc := range cases {
		if got := clamp01(tc.value); got != tc.expected {
			t.Fatalf("clamp01(%f) = %f, want %f", tc.value, got, tc.expected)
		}
	}
}
