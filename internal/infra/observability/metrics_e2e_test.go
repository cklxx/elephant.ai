//go:build integration

package observability

import (
	"testing"
	"time"

	"alex/internal/shared/modelregistry"
)

func waitForMetricsRegistry(t *testing.T) {
	t.Helper()
	if !modelregistry.WaitUntilReady(20 * time.Second) {
		t.Fatal("modelregistry did not load from models.dev within 20 s")
	}
}

// registryInfoWithPricing returns the ModelInfo only if the model is in the
// registry AND has non-zero pricing. Used to guard test cases.
func registryInfoWithPricing(modelID string) (modelregistry.ModelInfo, bool) {
	info, ok := modelregistry.Lookup(modelID)
	if !ok || info.InputPer1M <= 0 {
		return modelregistry.ModelInfo{}, false
	}
	return info, true
}

// TestEstimateCost_RealRegistry verifies that EstimateCost returns values
// sourced from the live registry rather than the old hardcoded map.
func TestEstimateCost_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForMetricsRegistry(t)

	candidates := []struct {
		model        string
		inputTokens  int
		outputTokens int
	}{
		// Claude 4.x — latest generation (released 2025-10 ~ 2026-02)
		{"claude-sonnet-4-6", 100_000, 20_000},
		{"claude-opus-4-6", 50_000, 10_000},
		{"claude-haiku-4-5", 100_000, 20_000},
		{"claude-haiku-4-5-20251001", 100_000, 20_000},
		// GPT-5.x series (released 2025-08 ~ 2026-02)
		{"gpt-5.3-codex", 50_000, 10_000},
		{"gpt-5.2", 50_000, 10_000},
		{"gpt-5.1", 50_000, 10_000},
		{"gpt-5", 50_000, 10_000},
		{"gpt-5-mini", 100_000, 20_000},
		{"gpt-5-nano", 100_000, 20_000},
		// GPT-4.1 family (released 2025-04, 1M context)
		{"gpt-4.1", 50_000, 10_000},
		{"gpt-4.1-mini", 100_000, 20_000},
		{"gpt-4.1-nano", 100_000, 20_000},
		// Reasoning models
		{"o4-mini", 50_000, 10_000},
		{"o3", 50_000, 10_000},
		// Legacy stable (baseline)
		{"claude-3-5-sonnet-20241022", 100_000, 20_000},
		{"gpt-4o", 50_000, 10_000},
	}

	anyHit := false
	for _, tc := range candidates {
		info, ok := registryInfoWithPricing(tc.model)
		if !ok {
			t.Logf("SKIP  %-45s — not in registry or no pricing data", tc.model)
			continue
		}
		anyHit = true

		cost := EstimateCost(tc.model, tc.inputTokens, tc.outputTokens)

		// Expected cost based on registry data (same formula as EstimateCost).
		expectedCost := float64(tc.inputTokens)/1e6*info.InputPer1M +
			float64(tc.outputTokens)/1e6*info.OutputPer1M

		t.Logf("EstimateCost(%-45q, %6d in, %5d out) = $%.6f  (expected $%.6f from registry $%.4f/$%.4f per 1M)",
			tc.model, tc.inputTokens, tc.outputTokens, cost, expectedCost, info.InputPer1M, info.OutputPer1M)

		if cost <= 0 {
			t.Errorf("%s: EstimateCost should be positive", tc.model)
		}
		// Cost must match what registry data says (within floating-point epsilon).
		if cost-expectedCost > 1e-9 {
			t.Errorf("%s: EstimateCost=$%.8f, want $%.8f from registry", tc.model, cost, expectedCost)
		}
		// Sanity: total tokens * max plausible price should be under $1000.
		totalTokens := tc.inputTokens + tc.outputTokens
		if cost > float64(totalTokens)/1e6*1000 {
			t.Errorf("%s: cost $%f looks implausibly large for %d tokens", tc.model, cost, totalTokens)
		}
	}

	if !anyHit {
		t.Error("none of the candidate models had pricing data in models.dev — check network or model IDs")
	}
}

// TestEstimateCost_FallbackForUnknown_RealRegistry verifies the $1.5/1M blended
// fallback is used when the model is not in the registry.
func TestEstimateCost_FallbackForUnknown_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForMetricsRegistry(t)

	const model = "definitely-not-a-real-model-xyz-9999"
	const inputTokens = 1_000_000
	const outputTokens = 0

	cost := EstimateCost(model, inputTokens, outputTokens)
	t.Logf("EstimateCost(unknown, 1M in, 0 out) = $%.6f  (expected fallback $1.50)", cost)

	// Fallback = (1M + 0) / 1M * $1.5 = $1.50
	const expected = 1.5
	const tolerance = 1e-9
	if cost < expected-tolerance || cost > expected+tolerance {
		t.Errorf("fallback cost = $%f, want $%.2f", cost, expected)
	}
}

// TestEstimateCost_RegistryCostsMoreForPremiumModels verifies that premium
// models (higher price per token) cost more than budget models for the same
// token count.
func TestEstimateCost_RegistryCostsMoreForPremiumModels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForMetricsRegistry(t)

	// Pick one premium and one budget model both expected in the registry.
	premiumCandidates := []string{"claude-opus-4-6", "gpt-5-pro", "gpt-5.2-pro", "claude-3-opus-20240229", "gpt-4o"}
	budgetCandidates := []string{"gpt-5-nano", "claude-haiku-4-5", "gpt-4.1-nano", "gpt-4o-mini"}

	var premium, budget string
	var premiumInfo, budgetInfo modelregistry.ModelInfo
	for _, m := range premiumCandidates {
		if info, ok := registryInfoWithPricing(m); ok {
			premium = m
			premiumInfo = info
			break
		}
	}
	for _, m := range budgetCandidates {
		if info, ok := registryInfoWithPricing(m); ok {
			budget = m
			budgetInfo = info
			break
		}
	}

	if premium == "" || budget == "" {
		t.Skipf("SKIP — need at least one premium and one budget model in registry (premium=%q budget=%q)",
			premium, budget)
	}

	// A premium model must cost more per 1M tokens than a budget model.
	premiumPer1M := premiumInfo.InputPer1M + premiumInfo.OutputPer1M
	budgetPer1M := budgetInfo.InputPer1M + budgetInfo.OutputPer1M

	t.Logf("Premium %s: $%.4f+$%.4f = $%.4f/1M", premium, premiumInfo.InputPer1M, premiumInfo.OutputPer1M, premiumPer1M)
	t.Logf("Budget  %s: $%.4f+$%.4f = $%.4f/1M", budget, budgetInfo.InputPer1M, budgetInfo.OutputPer1M, budgetPer1M)

	const tokens = 100_000
	premiumCost := EstimateCost(premium, tokens, tokens)
	budgetCost := EstimateCost(budget, tokens, tokens)

	t.Logf("EstimateCost(%s, 100k, 100k) = $%.6f", premium, premiumCost)
	t.Logf("EstimateCost(%s, 100k, 100k) = $%.6f", budget, budgetCost)

	if premiumCost <= budgetCost {
		t.Errorf("expected premium %s ($%f) to cost more than budget %s ($%f)",
			premium, premiumCost, budget, budgetCost)
	}
}
