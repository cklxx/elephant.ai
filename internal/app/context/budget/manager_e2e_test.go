//go:build integration

package budget

import (
	"testing"
	"time"

	"alex/internal/shared/modelregistry"
)

func waitForBudgetRegistry(t *testing.T) {
	t.Helper()
	if !modelregistry.WaitUntilReady(20 * time.Second) {
		t.Fatal("modelregistry did not load from models.dev within 20 s")
	}
}

// TestEstimateCost_RealRegistry verifies that estimateCost (via
// storage.GetModelPricing) returns registry-sourced values for known models.
func TestEstimateCost_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForBudgetRegistry(t)

	type tc struct {
		model        string
		inputTokens  int
		outputTokens int
		wantMin      float64
		wantMax      float64
	}

	cases := []tc{
		// Claude 4.x latest
		{
			// claude-sonnet-4-6: $3/1M in, $15/1M out (released 2026-02-17)
			model:        "claude-sonnet-4-6",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0001,
			wantMax:      0.5,
		},
		{
			// claude-opus-4-6: $5/1M in, $25/1M out (released 2026-02-05)
			model:        "claude-opus-4-6",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0001,
			wantMax:      1.0,
		},
		{
			// claude-haiku-4-5: $1/1M in, $5/1M out (released 2025-10-15)
			model:        "claude-haiku-4-5",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.000001,
			wantMax:      0.2,
		},
		// GPT-5.x series
		{
			// gpt-5: $1.25/1M in, $10/1M out (released 2025-08-07)
			model:        "gpt-5",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0001,
			wantMax:      0.5,
		},
		{
			// gpt-5-mini: $0.25/1M in, $2/1M out (released 2025-08-07)
			model:        "gpt-5-mini",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.000001,
			wantMax:      0.1,
		},
		{
			// gpt-5-nano: $0.05/1M in, $0.4/1M out (released 2025-08-07)
			model:        "gpt-5-nano",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0000001,
			wantMax:      0.05,
		},
		// GPT-4.1 family
		{
			// gpt-4.1: $2/1M in, $8/1M out, 1M context (released 2025-04-14)
			model:        "gpt-4.1",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0001,
			wantMax:      0.5,
		},
		{
			// gpt-4.1-nano: $0.1/1M in, $0.4/1M out (released 2025-04-14)
			model:        "gpt-4.1-nano",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0000001,
			wantMax:      0.05,
		},
		// Legacy stable (baseline)
		{
			// claude-3-5-sonnet-20241022: $3/1M in, $15/1M out
			model:        "claude-3-5-sonnet-20241022",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0001,
			wantMax:      0.5,
		},
		{
			// gpt-4o: $2.5/1M in, $10/1M out
			model:        "gpt-4o",
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantMin:      0.0001,
			wantMax:      0.5,
		},
	}

	anyHit := false
	for _, tc := range cases {
		if _, ok := modelregistry.Lookup(tc.model); !ok {
			t.Logf("SKIP  %s — not in registry", tc.model)
			continue
		}
		anyHit = true

		cost := estimateCost(tc.inputTokens, tc.outputTokens, tc.model, DefaultModelTiers)
		t.Logf("estimateCost(%-45q, %5d in, %4d out) = $%.8f", tc.model, tc.inputTokens, tc.outputTokens, cost)

		if cost < tc.wantMin || cost > tc.wantMax {
			t.Errorf("%s: cost=$%.8f, want in [$%.8f, $%.8f]",
				tc.model, cost, tc.wantMin, tc.wantMax)
		}
	}

	if !anyHit {
		t.Error("no models were found in the registry — models.dev may have changed model IDs")
	}
}

// TestBudgetManager_RealRegistryCost verifies RecordUsage accumulates realistic
// costs using registry pricing for a full session lifecycle.
func TestBudgetManager_RealRegistryCost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForBudgetRegistry(t)

	// Prefer the latest model; fall back to stable if not yet in registry.
	candidates := []string{"gpt-5.3-codex", "claude-sonnet-4-6", "gpt-5", "gpt-4o"}
	var model string
	for _, m := range candidates {
		if _, ok := modelregistry.Lookup(m); ok {
			model = m
			break
		}
	}
	if model == "" {
		t.Skip("no candidate models found in registry — skipping")
	}

	quota := SessionQuota{
		MaxCostUSD:       100.0, // generous limit so we don't trip it
		WarningThreshold: 0.8,
	}
	mgr := NewManager(quota, DefaultModelTiers)
	sid := "e2e-session-1"

	// Simulate 5 turns of 10k in / 2k out each.
	for i := 0; i < 5; i++ {
		mgr.RecordUsage(sid, 10_000, 2_000, model)
	}

	u := mgr.GetUsage(sid)
	t.Logf("After 5 turns of %s (10k in, 2k out each):", model)
	t.Logf("  InputTokens=%d  OutputTokens=%d  TotalTokens=%d", u.InputTokens, u.OutputTokens, u.TotalTokens)
	t.Logf("  EstimatedCostUSD=$%.6f  TurnCount=%d", u.EstimatedCostUSD, u.TurnCount)

	if u.TurnCount != 5 {
		t.Errorf("TurnCount = %d, want 5", u.TurnCount)
	}
	if u.InputTokens != 50_000 {
		t.Errorf("InputTokens = %d, want 50000", u.InputTokens)
	}
	if u.EstimatedCostUSD <= 0 {
		t.Error("EstimatedCostUSD should be positive with registry pricing")
	}
	// 5 turns × (10k in + 2k out) for any current model: expect $0.001–$20
	if u.EstimatedCostUSD < 0.001 || u.EstimatedCostUSD > 20.0 {
		t.Errorf("EstimatedCostUSD=$%.6f looks implausible for 50k/10k %s tokens", u.EstimatedCostUSD, model)
	}
}

// TestBudgetManager_PremiumCostsMoreThanBudget verifies that the cost for a
// premium model exceeds the cost for a budget model for the same token usage.
func TestBudgetManager_PremiumCostsMoreThanBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForBudgetRegistry(t)

	// Use latest premium vs latest budget models from models.dev.
	premium := "claude-opus-4-6"   // $5/$25 per 1M (released 2026-02-05)
	budget := "gpt-5-nano"         // $0.05/$0.4 per 1M (released 2025-08-07)

	_, hasPremium := modelregistry.Lookup(premium)
	_, hasBudget := modelregistry.Lookup(budget)
	if !hasPremium || !hasBudget {
		t.Skipf("SKIP — one of the models not in registry (premium=%v budget=%v)", hasPremium, hasBudget)
	}

	quota := SessionQuota{MaxCostUSD: 1000.0, WarningThreshold: 0.8}

	mPremium := NewManager(quota, DefaultModelTiers)
	mBudget := NewManager(quota, DefaultModelTiers)

	const input, output = 20_000, 5_000
	mPremium.RecordUsage("s1", input, output, premium)
	mBudget.RecordUsage("s2", input, output, budget)

	uP := mPremium.GetUsage("s1")
	uB := mBudget.GetUsage("s2")

	t.Logf("%s: $%.6f for %dk in / %dk out", premium, uP.EstimatedCostUSD, input/1000, output/1000)
	t.Logf("%s: $%.6f for %dk in / %dk out", budget, uB.EstimatedCostUSD, input/1000, output/1000)

	if uP.EstimatedCostUSD <= uB.EstimatedCostUSD {
		t.Errorf("expected premium model ($%f) to cost more than budget ($%f)", uP.EstimatedCostUSD, uB.EstimatedCostUSD)
	}
}
