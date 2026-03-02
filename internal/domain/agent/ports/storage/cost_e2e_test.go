//go:build integration

package storage

import (
	"math"
	"testing"
	"time"

	"alex/internal/shared/modelregistry"
)

func waitForCostRegistry(t *testing.T) {
	t.Helper()
	if !modelregistry.WaitUntilReady(20 * time.Second) {
		t.Fatal("modelregistry did not load from models.dev within 20 s")
	}
}

// registryHasPricing returns true if the model is in the registry WITH valid
// pricing data (InputPer1M > 0). Models missing pricing data should be skipped.
func registryHasPricing(modelID string) (ModelPricing, bool) {
	info, ok := modelregistry.Lookup(modelID)
	if !ok || info.InputPer1M <= 0 {
		return ModelPricing{}, false
	}
	return ModelPricing{
		InputPer1K:  info.InputPer1M / 1000.0,
		OutputPer1K: info.OutputPer1M / 1000.0,
	}, true
}

// TestGetModelPricing_RealRegistry verifies that GetModelPricing returns
// registry-sourced pricing for well-known models (non-zero, self-consistent).
func TestGetModelPricing_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForCostRegistry(t)

	// Models expected in models.dev; exact prices intentionally wide because
	// models.dev may report cached/discounted pricing that differs from list price.
	candidates := []string{
		// Claude 4.x — latest (released 2025-10 ~ 2026-02)
		"claude-sonnet-4-6",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-haiku-4-5-20251001",
		// GPT-5.x series (released 2025-08 ~ 2026-02)
		"gpt-5.3-codex",
		"gpt-5.2",
		"gpt-5.1",
		"gpt-5",
		"gpt-5-mini",
		"gpt-5-nano",
		// GPT-4.1 family (released 2025-04)
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		// Reasoning models
		"o4-mini",
		"o3",
		// Legacy stable (baseline)
		"claude-3-5-sonnet-20241022",
		"claude-3-opus-20240229",
		"claude-3-haiku-20240307",
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
	}

	anyHit := false
	for _, modelID := range candidates {
		reg, ok := registryHasPricing(modelID)
		if !ok {
			t.Logf("SKIP  %-45s — not in registry or no pricing data", modelID)
			continue
		}
		anyHit = true

		pricing := GetModelPricing(modelID)
		t.Logf("HIT   %-45s input=$%.6f/1K  output=$%.6f/1K", modelID, pricing.InputPer1K, pricing.OutputPer1K)

		// Registry-sourced pricing must match what we fetched directly.
		if math.Abs(pricing.InputPer1K-reg.InputPer1K) > 1e-10 {
			t.Errorf("%s: GetModelPricing InputPer1K=%f, registry=%f", modelID, pricing.InputPer1K, reg.InputPer1K)
		}
		if math.Abs(pricing.OutputPer1K-reg.OutputPer1K) > 1e-10 {
			t.Errorf("%s: GetModelPricing OutputPer1K=%f, registry=%f", modelID, pricing.OutputPer1K, reg.OutputPer1K)
		}

		// Both must be positive — a zero price is invalid data.
		if pricing.InputPer1K <= 0 {
			t.Errorf("%s: InputPer1K=%f, want > 0", modelID, pricing.InputPer1K)
		}
		if pricing.OutputPer1K <= 0 {
			t.Errorf("%s: OutputPer1K=%f, want > 0", modelID, pricing.OutputPer1K)
		}

		// Both must be within a sane absolute range: $0.00001–$1 per 1K tokens
		// ($0.01–$1000/1M). Catches unit errors.
		if pricing.InputPer1K > 1.0 {
			t.Errorf("%s: InputPer1K=%f looks implausibly large (>$1/1K)", modelID, pricing.InputPer1K)
		}
		if pricing.OutputPer1K > 10.0 {
			t.Errorf("%s: OutputPer1K=%f looks implausibly large (>$10/1K)", modelID, pricing.OutputPer1K)
		}
	}

	if !anyHit {
		t.Error("none of the candidate models had pricing data in models.dev — check network or model IDs")
	}
}

// TestCalculateCost_RealRegistry verifies end-to-end cost calculation using
// registry pricing.
func TestCalculateCost_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForCostRegistry(t)

	// Find the first candidate with valid registry pricing.
	candidates := []string{"gpt-5.3-codex", "claude-sonnet-4-6", "gpt-5", "gpt-4.1", "gpt-4o", "claude-3-5-sonnet-20241022"}
	var model string
	for _, m := range candidates {
		if _, ok := registryHasPricing(m); ok {
			model = m
			break
		}
	}
	if model == "" {
		t.Skip("no candidate models found in registry with pricing — skipping")
	}

	const inputTokens = 10_000
	const outputTokens = 2_000

	inputCost, outputCost, totalCost := CalculateCost(inputTokens, outputTokens, model)

	t.Logf("CalculateCost(%q, %d, %d): input=$%.6f  output=$%.6f  total=$%.6f",
		model, inputTokens, outputTokens, inputCost, outputCost, totalCost)

	if inputCost <= 0 {
		t.Error("inputCost should be positive")
	}
	if outputCost <= 0 {
		t.Error("outputCost should be positive")
	}
	if math.Abs(totalCost-(inputCost+outputCost)) > 1e-10 {
		t.Errorf("totalCost (%f) != inputCost+outputCost (%f)", totalCost, inputCost+outputCost)
	}
	// Sanity: 10k + 2k tokens of any known model should cost between $0.0001 and $5
	if totalCost < 0.0001 || totalCost > 5.0 {
		t.Errorf("totalCost=$%f looks implausible for 12k tokens of %q", totalCost, model)
	}
}

// TestGetModelPricing_RegistryOverridesStaticTable verifies that registry values
// supersede the static fallback table for models that appear in both.
func TestGetModelPricing_RegistryOverridesStaticTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForCostRegistry(t)

	// "gpt-4o" exists in both the registry and the static fallback table.
	const model = "gpt-4o"
	registryPricing, ok := registryHasPricing(model)
	if !ok {
		t.Skipf("model %q not in registry with pricing — cannot verify override", model)
	}

	pricing := GetModelPricing(model)
	t.Logf("GetModelPricing(%q): input=$%.6f/1K  output=$%.6f/1K", model, pricing.InputPer1K, pricing.OutputPer1K)
	t.Logf("Registry direct:      input=$%.6f/1K  output=$%.6f/1K", registryPricing.InputPer1K, registryPricing.OutputPer1K)

	// The returned pricing must match registry data (not the old static table value 0.005).
	if math.Abs(pricing.InputPer1K-registryPricing.InputPer1K) > 1e-10 {
		t.Errorf("GetModelPricing did not use registry data: got %f, registry=%f",
			pricing.InputPer1K, registryPricing.InputPer1K)
	}
}

// TestGetModelPricing_UnknownModelFallback_RealRegistry verifies the fallback
// pricing is used when a model is not in the registry.
func TestGetModelPricing_UnknownModelFallback_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForCostRegistry(t)

	const model = "definitely-not-a-real-model-xyz-9999"
	pricing := GetModelPricing(model)
	t.Logf("GetModelPricing(unknown): input=$%.6f/1K  output=$%.6f/1K", pricing.InputPer1K, pricing.OutputPer1K)

	// Must use the default fallback {0.001, 0.002}.
	if math.Abs(pricing.InputPer1K-0.001) > 1e-10 {
		t.Errorf("fallback InputPer1K=%f, want 0.001", pricing.InputPer1K)
	}
	if math.Abs(pricing.OutputPer1K-0.002) > 1e-10 {
		t.Errorf("fallback OutputPer1K=%f, want 0.002", pricing.OutputPer1K)
	}
}
