//go:build integration

package modelregistry

import (
	"net/http"
	"testing"
	"time"
)

// knownAnthropicModels are model IDs expected to be in models.dev under the
// "anthropic" provider. Includes both legacy stable models and the latest
// claude-4.x generation.
var knownAnthropicModels = []string{
	// Legacy stable
	"claude-3-5-sonnet-20241022",
	"claude-3-5-sonnet-20240620",
	"claude-3-opus-20240229",
	"claude-3-haiku-20240307",
	// Claude 4.x — latest generation
	"claude-sonnet-4-5",
	"claude-sonnet-4-5-20250929",
	"claude-sonnet-4-6",
	"claude-opus-4-5",
	"claude-opus-4-5-20251101",
	"claude-opus-4-6",
	"claude-haiku-4-5",
	"claude-haiku-4-5-20251001",
}

var knownOpenAIModels = []string{
	// Stable GPT-4 family
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-4-turbo",
	"gpt-4",
	// GPT-5 generation
	"gpt-5",
	"gpt-5-mini",
	"gpt-5-nano",
	// Reasoning models
	"o4-mini",
	"o3",
	"o3-mini",
}

// TestRegistryFetch_RealAPI verifies that a fresh Registry can fetch and parse
// real data from models.dev/api.json.
func TestRegistryFetch_RealAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}

	reg := &Registry{client: &http.Client{Timeout: 15 * time.Second}}

	if !reg.WaitUntilReady(20 * time.Second) {
		t.Fatal("models.dev did not respond within 20 s — check network connectivity")
	}

	// Verify we got a meaningful number of models across providers.
	reg.mu.RLock()
	total := len(reg.data)
	reg.mu.RUnlock()

	t.Logf("registry loaded %d model entries (compound + bare keys)", total)
	if total < 10 {
		t.Errorf("expected at least 10 model entries, got %d", total)
	}
}

// TestRegistryLookup_AnthropicModels verifies Lookup returns valid metadata for
// well-known Anthropic models.
func TestRegistryLookup_AnthropicModels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}

	reg := &Registry{client: &http.Client{Timeout: 15 * time.Second}}
	if !reg.WaitUntilReady(20 * time.Second) {
		t.Fatal("models.dev did not respond within 20 s")
	}

	found := 0
	for _, modelID := range knownAnthropicModels {
		info, ok := reg.Lookup(modelID)
		if !ok {
			t.Logf("MISS  %s (not in models.dev — may have been added later)", modelID)
			continue
		}
		found++
		t.Logf("HIT   %s: context=%d  input=$%.4f/1M  output=$%.4f/1M",
			modelID, info.ContextWindow, info.InputPer1M, info.OutputPer1M)

		if info.ContextWindow <= 0 {
			t.Errorf("%s: ContextWindow = %d, want > 0", modelID, info.ContextWindow)
		}
		if info.InputPer1M <= 0 {
			t.Errorf("%s: InputPer1M = %f, want > 0", modelID, info.InputPer1M)
		}
		if info.OutputPer1M <= 0 {
			t.Errorf("%s: OutputPer1M = %f, want > 0", modelID, info.OutputPer1M)
		}
		if info.Provider == "" {
			t.Errorf("%s: Provider is empty", modelID)
		}
	}

	if found == 0 {
		t.Errorf("none of the expected Anthropic models were found in models.dev; got zero hits out of %d", len(knownAnthropicModels))
	}
}

// TestRegistryLookup_OpenAIModels verifies Lookup returns valid metadata for
// well-known OpenAI models.
func TestRegistryLookup_OpenAIModels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}

	reg := &Registry{client: &http.Client{Timeout: 15 * time.Second}}
	if !reg.WaitUntilReady(20 * time.Second) {
		t.Fatal("models.dev did not respond within 20 s")
	}

	found := 0
	for _, modelID := range knownOpenAIModels {
		info, ok := reg.Lookup(modelID)
		if !ok {
			t.Logf("MISS  %s", modelID)
			continue
		}
		found++
		t.Logf("HIT   %s: context=%d  input=$%.4f/1M  output=$%.4f/1M",
			modelID, info.ContextWindow, info.InputPer1M, info.OutputPer1M)

		if info.ContextWindow <= 0 {
			t.Errorf("%s: ContextWindow = %d, want > 0", modelID, info.ContextWindow)
		}
		if info.InputPer1M <= 0 {
			t.Errorf("%s: InputPer1M = %f, want > 0", modelID, info.InputPer1M)
		}
	}

	if found == 0 {
		t.Errorf("none of the expected OpenAI models were found in models.dev; got zero hits out of %d", len(knownOpenAIModels))
	}
}

// TestRegistryCompoundKey_E2E verifies that both "provider/model" and bare
// "model" lookups return the same data.
func TestRegistryCompoundKey_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}

	reg := &Registry{client: &http.Client{Timeout: 15 * time.Second}}
	if !reg.WaitUntilReady(20 * time.Second) {
		t.Fatal("models.dev did not respond within 20 s")
	}

	// Find at least one model that responds to a compound key lookup.
	type pair struct{ compound, bare string }
	pairs := []pair{
		{"anthropic/claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20241022"},
		{"openai/gpt-4o", "gpt-4o"},
		{"anthropic/claude-3-opus-20240229", "claude-3-opus-20240229"},
	}

	for _, p := range pairs {
		compound, okC := reg.Lookup(p.compound)
		bare, okB := reg.Lookup(p.bare)

		if !okC && !okB {
			t.Logf("SKIP  %s — not in models.dev", p.bare)
			continue
		}
		if okC && okB {
			if compound.ContextWindow != bare.ContextWindow {
				t.Errorf("%s: compound and bare lookups returned different ContextWindow (%d vs %d)",
					p.bare, compound.ContextWindow, bare.ContextWindow)
			}
			t.Logf("OK    %s: compound=%v bare=%v ctx=%d", p.bare, okC, okB, bare.ContextWindow)
		}
	}
}

// TestRegistryProviderModels_E2E verifies ProviderModels returns a non-empty
// slice for major providers.
func TestRegistryProviderModels_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}

	reg := &Registry{client: &http.Client{Timeout: 15 * time.Second}}
	if !reg.WaitUntilReady(20 * time.Second) {
		t.Fatal("models.dev did not respond within 20 s")
	}

	for _, provider := range []string{"anthropic", "openai"} {
		models := reg.ProviderModels(provider)
		t.Logf("provider %q: %d models", provider, len(models))
		if len(models) == 0 {
			t.Errorf("ProviderModels(%q) returned empty; expected known models from models.dev", provider)
		}
		if len(models) > 5 {
			t.Logf("  first 5: %v", models[:5])
		} else {
			t.Logf("  all: %v", models)
		}
	}
}

// TestRegistryContextWindowRanges_E2E verifies context window values are in
// realistic ranges for the models found.
func TestRegistryContextWindowRanges_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}

	reg := &Registry{client: &http.Client{Timeout: 15 * time.Second}}
	if !reg.WaitUntilReady(20 * time.Second) {
		t.Fatal("models.dev did not respond within 20 s")
	}

	type check struct {
		model   string
		wantMin int
		wantMax int
	}
	checks := []check{
		// Legacy Claude — all 200k
		{"claude-3-5-sonnet-20241022", 100_000, 250_000},
		{"claude-3-opus-20240229", 100_000, 250_000},
		// Claude 4.x — 200k
		{"claude-sonnet-4-6", 100_000, 250_000},
		{"claude-opus-4-6", 100_000, 250_000},
		{"claude-haiku-4-5", 100_000, 250_000},
		// GPT-4o family — 128k
		{"gpt-4o", 64_000, 200_000},
		{"gpt-4o-mini", 64_000, 200_000},
		// GPT-5 family — 400k
		{"gpt-5", 300_000, 500_000},
		{"gpt-5-mini", 300_000, 500_000},
		// Reasoning models — 200k
		{"o4-mini", 100_000, 250_000},
		{"o3", 100_000, 250_000},
	}

	for _, c := range checks {
		info, ok := reg.Lookup(c.model)
		if !ok {
			t.Logf("SKIP  %s — not in models.dev", c.model)
			continue
		}
		t.Logf("CHECK %s: ContextWindow=%d (want %d–%d)", c.model, info.ContextWindow, c.wantMin, c.wantMax)
		if info.ContextWindow < c.wantMin || info.ContextWindow > c.wantMax {
			t.Errorf("%s: ContextWindow=%d, want in [%d, %d]",
				c.model, info.ContextWindow, c.wantMin, c.wantMax)
		}
	}
}
