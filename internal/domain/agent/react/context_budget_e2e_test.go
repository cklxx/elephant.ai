//go:build integration

package react

import (
	"testing"
	"time"

	"alex/internal/shared/modelregistry"
)

// waitForDefaultRegistry blocks until the modelregistry.Default has data.
func waitForDefaultRegistry(t *testing.T) {
	t.Helper()
	if !modelregistry.WaitUntilReady(20 * time.Second) {
		t.Fatal("modelregistry did not load from models.dev within 20 s — check network connectivity")
	}
}

// TestModelContextWindowTokens_RealRegistry verifies that modelContextWindowTokens
// returns values sourced from the live models.dev registry when the data is present,
// and that the fallback still applies for unknown models.
func TestModelContextWindowTokens_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForDefaultRegistry(t)

	type tc struct {
		model      string
		wantMin    int
		wantMax    int
		description string
	}

	cases := []tc{
		// Claude 4.x — latest generation, all 200k context
		{
			model:       "claude-sonnet-4-6",
			wantMin:     150_000,
			wantMax:     250_000,
			description: "claude-sonnet-4-6 (released 2026-02-17)",
		},
		{
			model:       "claude-opus-4-6",
			wantMin:     150_000,
			wantMax:     250_000,
			description: "claude-opus-4-6 (released 2026-02-05)",
		},
		{
			model:       "claude-haiku-4-5",
			wantMin:     150_000,
			wantMax:     250_000,
			description: "claude-haiku-4-5 (released 2025-10-15)",
		},
		// GPT-5.x series — 400k context
		{
			model:       "gpt-5.3-codex",
			wantMin:     300_000,
			wantMax:     500_000,
			description: "gpt-5.3-codex (released 2026-02-05)",
		},
		{
			model:       "gpt-5",
			wantMin:     300_000,
			wantMax:     500_000,
			description: "gpt-5 (released 2025-08-07)",
		},
		{
			model:       "gpt-5-mini",
			wantMin:     300_000,
			wantMax:     500_000,
			description: "gpt-5-mini (released 2025-08-07)",
		},
		// GPT-4.1 family — 1M context
		{
			model:       "gpt-4.1",
			wantMin:     500_000,
			wantMax:     2_000_000,
			description: "gpt-4.1 (released 2025-04-14, 1M context)",
		},
		// Reasoning models — 200k context
		{
			model:       "o4-mini",
			wantMin:     150_000,
			wantMax:     250_000,
			description: "o4-mini reasoning model (released 2025-04-16)",
		},
		{
			model:       "o3",
			wantMin:     150_000,
			wantMax:     250_000,
			description: "o3 reasoning model (released 2025-04-16)",
		},
		// Legacy stable (baseline)
		{
			model:       "claude-3-5-sonnet-20241022",
			wantMin:     150_000,
			wantMax:     250_000,
			description: "claude-3.5-sonnet (legacy stable)",
		},
		{
			model:       "gpt-4o",
			wantMin:     64_000,
			wantMax:     200_000,
			description: "gpt-4o (legacy stable)",
		},
	}

	anyHit := false
	for _, tc := range cases {
		got := modelContextWindowTokens(tc.model)
		t.Logf("%-45s → %d tokens (%s)", tc.model, got, tc.description)

		if _, ok := modelregistry.Lookup(tc.model); !ok {
			// Model not in registry — fallback value is acceptable.
			t.Logf("  (not in registry, using pattern-match fallback)")
			continue
		}

		anyHit = true
		if got < tc.wantMin || got > tc.wantMax {
			t.Errorf("%s: context window = %d, want in [%d, %d]",
				tc.model, got, tc.wantMin, tc.wantMax)
		}
	}

	// Verify at least one well-known model was found and returned registry data.
	if !anyHit {
		t.Error("no models were found in the registry — models.dev may have changed its model IDs")
	}
}

// TestModelContextWindowTokens_UnknownModel_RealRegistry confirms the fallback
// still applies for a model that is not in models.dev.
func TestModelContextWindowTokens_UnknownModel_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForDefaultRegistry(t)

	got := modelContextWindowTokens("definitely-not-a-real-model-xyz-9999")
	t.Logf("unknown model context window: %d (want %d)", got, defaultModelContextWindowTokens)
	if got != defaultModelContextWindowTokens {
		t.Errorf("unknown model = %d, want default %d", got, defaultModelContextWindowTokens)
	}
}

// TestDeriveContextTokenLimit_RealRegistry verifies that the full context budget
// derivation pipeline produces larger budgets for large-context models.
func TestDeriveContextTokenLimit_RealRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	waitForDefaultRegistry(t)

	// Pick two models from different context-window tiers.
	// gpt-5 has 400k vs claude-sonnet-4-6 has 200k — both in registry.
	largeModel := "gpt-5"           // 400k (released 2025-08-07)
	smallModel := "claude-sonnet-4-6" // 200k (released 2026-02-17)

	large := deriveContextTokenLimit(largeModel, 4096)
	small := deriveContextTokenLimit(smallModel, 4096)

	t.Logf("deriveContextTokenLimit(%q, 4096) = %d", largeModel, large)
	t.Logf("deriveContextTokenLimit(%q, 4096) = %d", smallModel, small)

	if _, okL := modelregistry.Lookup(largeModel); !okL {
		t.Logf("SKIP comparison — %s not in registry", largeModel)
		return
	}
	if _, okS := modelregistry.Lookup(smallModel); !okS {
		t.Logf("SKIP comparison — %s not in registry", smallModel)
		return
	}

	if large <= small {
		t.Errorf("expected large-context model budget (%d) > small-context (%d)", large, small)
	}
}
