package router

import (
	"context"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

func smallModel() ModelProfile {
	return ModelProfile{
		Provider:         "openai",
		Model:            "gpt-4o-mini",
		Tier:             TierSmall,
		MaxContextTokens: 128_000,
		CostPer1KInput:   0.00015,
		CostPer1KOutput:  0.0006,
		AvgLatencyMs:     200,
		Capabilities:     []string{"code", "reasoning"},
	}
}

func defaultModel() ModelProfile {
	return ModelProfile{
		Provider:         "openai",
		Model:            "gpt-4o",
		Tier:             TierDefault,
		MaxContextTokens: 128_000,
		CostPer1KInput:   0.005,
		CostPer1KOutput:  0.015,
		AvgLatencyMs:     800,
		Capabilities:     []string{"code", "reasoning", "vision"},
	}
}

func strongModel() ModelProfile {
	return ModelProfile{
		Provider:         "anthropic",
		Model:            "claude-opus-4-5-20251101",
		Tier:             TierStrong,
		MaxContextTokens: 200_000,
		CostPer1KInput:   0.015,
		CostPer1KOutput:  0.075,
		AvgLatencyMs:     2000,
		Capabilities:     []string{"code", "reasoning", "vision"},
	}
}

func cheapDefault() ModelProfile {
	return ModelProfile{
		Provider:         "deepseek",
		Model:            "deepseek-chat",
		Tier:             TierDefault,
		MaxContextTokens: 64_000,
		CostPer1KInput:   0.001,
		CostPer1KOutput:  0.002,
		AvgLatencyMs:     600,
		Capabilities:     []string{"code", "reasoning"},
	}
}

func allModels() []ModelProfile {
	return []ModelProfile{smallModel(), defaultModel(), strongModel(), cheapDefault()}
}

func newTestRouter() *Router {
	return NewRouter(RouterConfig{
		Models:      allModels(),
		DefaultTier: TierDefault,
		CostWeight:  0.3,
		LatencyWeight: 0.2,
	})
}

// ---------------------------------------------------------------------------
// Route tests
// ---------------------------------------------------------------------------

func TestRoute_SimpleTaskPrefersSmallTier(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "simple",
	})
	if result.Profile.Tier != TierSmall {
		t.Errorf("expected TierSmall, got %s (model=%s)", result.Profile.Tier, result.Profile.Model)
	}
	if result.Profile.Model != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %s", result.Profile.Model)
	}
}

func TestRoute_ComplexTaskPrefersStrongTier(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "complex",
	})
	if result.Profile.Tier != TierStrong {
		t.Errorf("expected TierStrong, got %s (model=%s)", result.Profile.Tier, result.Profile.Model)
	}
}

func TestRoute_FiltersByRequiredCapabilities(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		RequiredCapabilities: []string{"vision"},
	})
	// Only defaultModel and strongModel have "vision".
	// Default tier is preferred when no complexity signal.
	if !result.Profile.hasCapability("vision") {
		t.Errorf("expected model with vision, got %s (caps=%v)", result.Profile.Model, result.Profile.Capabilities)
	}
	// smallModel and cheapDefault should be excluded from the result and fallbacks.
	for _, fb := range result.Fallbacks {
		if !fb.hasCapability("vision") {
			t.Errorf("fallback %s lacks required capability 'vision'", fb.Model)
		}
	}
}

func TestRoute_FiltersByMaxContextTokens(t *testing.T) {
	r := newTestRouter()
	// Request with 150K tokens should exclude models with <= 128K context.
	result := r.Route(context.Background(), RoutingRequest{
		EstimatedTokens: 150_000,
	})
	// Only strongModel (200K) should qualify.
	if result.Profile.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected claude-opus-4-5-20251101 (200K ctx), got %s (ctx=%d)",
			result.Profile.Model, result.Profile.MaxContextTokens)
	}
}

func TestRoute_FiltersUnhealthyProviders(t *testing.T) {
	r := newTestRouter()
	r.SetProviderHealth("openai", false)

	result := r.Route(context.Background(), RoutingRequest{})
	// openai models (gpt-4o-mini, gpt-4o) should be excluded.
	if result.Profile.Provider == "openai" {
		t.Errorf("expected non-openai provider, got %s:%s", result.Profile.Provider, result.Profile.Model)
	}
	for _, fb := range result.Fallbacks {
		if fb.Provider == "openai" {
			t.Errorf("fallback should not include unhealthy provider openai, got %s:%s", fb.Provider, fb.Model)
		}
	}
}

func TestRoute_RespectsCostBudget(t *testing.T) {
	r := newTestRouter()
	// Set a very tight budget that only the small model can meet.
	// smallModel: 0.00015/1K input + 0.0006/1K output.
	// For 10K tokens: input cost = 10*0.00015 = 0.0015, output est (2.5K) = 2.5*0.0006 = 0.0015, total ~0.003
	result := r.Route(context.Background(), RoutingRequest{
		EstimatedTokens:   10_000,
		MaxCostPerRequest: 0.005,
	})
	if result.Profile.Model != "gpt-4o-mini" {
		t.Errorf("expected cheapest model gpt-4o-mini under budget, got %s", result.Profile.Model)
	}
}

func TestRoute_BuildsFallbackList(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "simple",
	})
	if len(result.Fallbacks) == 0 {
		t.Fatal("expected at least one fallback model")
	}
	// Primary is small tier → fallbacks should prefer TierDefault first, then TierStrong.
	for _, fb := range result.Fallbacks {
		if fb.Provider == result.Profile.Provider && fb.Model == result.Profile.Model {
			t.Errorf("primary model should not appear in fallbacks")
		}
	}
	// First fallback should be default tier (next tier up from small).
	if result.Fallbacks[0].Tier != TierDefault {
		t.Errorf("expected first fallback tier=default, got %s", result.Fallbacks[0].Tier)
	}
}

func TestRoute_NoMatchingModelsFallsBackToDefault(t *testing.T) {
	r := newTestRouter()
	// Require a capability nobody has.
	result := r.Route(context.Background(), RoutingRequest{
		RequiredCapabilities: []string{"quantum_computing"},
	})
	if result.Reason != "no_match_fallback_to_default" {
		t.Errorf("expected reason no_match_fallback_to_default, got %s", result.Reason)
	}
	if result.Profile.Tier != TierDefault {
		t.Errorf("expected default tier in fallback, got %s", result.Profile.Tier)
	}
}

func TestRoute_PreferredTierOverridesComplexity(t *testing.T) {
	r := newTestRouter()
	// Task says "complex" but preferred tier says "small".
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "complex",
		PreferredTier:  TierSmall,
	})
	if result.Profile.Tier != TierSmall {
		t.Errorf("expected TierSmall (preferred), got %s", result.Profile.Tier)
	}
}

func TestRoute_DefaultTierWhenNoSignal(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{})
	if result.Profile.Tier != TierDefault {
		t.Errorf("expected TierDefault when no signal, got %s", result.Profile.Tier)
	}
}

// ---------------------------------------------------------------------------
// RegisterModel tests
// ---------------------------------------------------------------------------

func TestRegisterModel_AddsNew(t *testing.T) {
	r := NewRouter(RouterConfig{
		Models: []ModelProfile{smallModel()},
	})
	before := r.AvailableModels(TierStrong)
	if len(before) != 0 {
		t.Fatalf("expected 0 strong models before register, got %d", len(before))
	}

	r.RegisterModel(strongModel())
	after := r.AvailableModels(TierStrong)
	if len(after) != 1 {
		t.Fatalf("expected 1 strong model after register, got %d", len(after))
	}
	if after[0].Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected claude-opus-4-5-20251101, got %s", after[0].Model)
	}
}

func TestRegisterModel_UpdatesExisting(t *testing.T) {
	r := NewRouter(RouterConfig{
		Models: []ModelProfile{smallModel()},
	})
	updated := smallModel()
	updated.AvgLatencyMs = 100 // faster
	r.RegisterModel(updated)

	models := r.AvailableModels(TierSmall)
	if len(models) != 1 {
		t.Fatalf("expected 1 small model, got %d", len(models))
	}
	if models[0].AvgLatencyMs != 100 {
		t.Errorf("expected updated latency 100, got %.0f", models[0].AvgLatencyMs)
	}
}

// ---------------------------------------------------------------------------
// AvailableModels tests
// ---------------------------------------------------------------------------

func TestAvailableModels_ListsCorrectTier(t *testing.T) {
	r := newTestRouter()
	smalls := r.AvailableModels(TierSmall)
	if len(smalls) != 1 {
		t.Errorf("expected 1 small model, got %d", len(smalls))
	}
	defaults := r.AvailableModels(TierDefault)
	if len(defaults) != 2 {
		t.Errorf("expected 2 default models, got %d", len(defaults))
	}
	strongs := r.AvailableModels(TierStrong)
	if len(strongs) != 1 {
		t.Errorf("expected 1 strong model, got %d", len(strongs))
	}
}

// ---------------------------------------------------------------------------
// SetProviderHealth tests
// ---------------------------------------------------------------------------

func TestSetProviderHealth_ExcludesUnhealthy(t *testing.T) {
	r := newTestRouter()

	// Mark anthropic unhealthy.
	r.SetProviderHealth("anthropic", false)

	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "complex",
	})
	if result.Profile.Provider == "anthropic" {
		t.Errorf("anthropic should be excluded when unhealthy")
	}

	// Re-enable anthropic.
	r.SetProviderHealth("anthropic", true)
	result = r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "complex",
	})
	if result.Profile.Provider != "anthropic" {
		t.Errorf("expected anthropic after re-enabling health, got %s", result.Profile.Provider)
	}
}

// ---------------------------------------------------------------------------
// Scoring weight tests
// ---------------------------------------------------------------------------

func TestScoring_CostHeavyPrefersCheaper(t *testing.T) {
	r := NewRouter(RouterConfig{
		Models:        allModels(),
		DefaultTier:   TierDefault,
		CostWeight:    0.8,
		LatencyWeight: 0.1,
	})
	result := r.Route(context.Background(), RoutingRequest{})
	// With cost weight 0.8, the cheapest default model (deepseek-chat) should win.
	if result.Profile.Model != "deepseek-chat" {
		t.Errorf("cost-heavy config should prefer deepseek-chat, got %s", result.Profile.Model)
	}
}

func TestScoring_LatencyHeavyPrefersFaster(t *testing.T) {
	// Among default-tier models: gpt-4o (800ms) vs deepseek-chat (600ms).
	r := NewRouter(RouterConfig{
		Models:        allModels(),
		DefaultTier:   TierDefault,
		CostWeight:    0.05,
		LatencyWeight: 0.9,
	})
	result := r.Route(context.Background(), RoutingRequest{})
	// With latency weight 0.9, the fastest default model should win.
	if result.Profile.Model != "deepseek-chat" {
		t.Errorf("latency-heavy config should prefer deepseek-chat (600ms), got %s (%.0fms)",
			result.Profile.Model, result.Profile.AvgLatencyMs)
	}
}

func TestScoring_CapabilityHeavyPrefersStronger(t *testing.T) {
	// Among default-tier models: gpt-4o has more capabilities (vision) and higher tier proxy.
	// With near-zero cost and latency weight, capability (tier rank) dominates.
	r := NewRouter(RouterConfig{
		Models: []ModelProfile{defaultModel(), cheapDefault()},
		DefaultTier:   TierDefault,
		CostWeight:    0.0,
		LatencyWeight: 0.0,
	})
	result := r.Route(context.Background(), RoutingRequest{})
	// Both are TierDefault, same tier rank. With all weights at 0, cap weight = 1.0,
	// but same tier → same cap score. Tie-break goes to first candidate.
	// This just verifies the scoring path doesn't panic with extreme weights.
	if result.Profile.Tier != TierDefault {
		t.Errorf("expected default tier, got %s", result.Profile.Tier)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestRoute_EmptyRouter(t *testing.T) {
	r := NewRouter(RouterConfig{})
	result := r.Route(context.Background(), RoutingRequest{})
	if result.Reason != "no_models_available" {
		t.Errorf("expected reason no_models_available, got %s", result.Reason)
	}
}

func TestRoute_SingleModel(t *testing.T) {
	r := NewRouter(RouterConfig{
		Models: []ModelProfile{defaultModel()},
	})
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "complex",
	})
	// Only one model available — it should be selected regardless of tier mismatch.
	if result.Profile.Model != "gpt-4o" {
		t.Errorf("expected gpt-4o as only model, got %s", result.Profile.Model)
	}
}

func TestRoute_AllProvidersUnhealthy(t *testing.T) {
	r := newTestRouter()
	r.SetProviderHealth("openai", false)
	r.SetProviderHealth("anthropic", false)
	r.SetProviderHealth("deepseek", false)

	result := r.Route(context.Background(), RoutingRequest{})
	// All filtered out → fallback to default.
	if result.Reason != "no_match_fallback_to_default" {
		t.Errorf("expected no_match_fallback_to_default, got %s", result.Reason)
	}
}

func TestRoute_ZeroEstimatedTokensSkipsContextFilter(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		EstimatedTokens: 0,
	})
	// Zero tokens should not filter anything.
	if result.Profile.Model == "" {
		t.Error("expected a model to be selected with zero estimated tokens")
	}
}

func TestRoute_CostBudgetZeroMeansNoLimit(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity:    "complex",
		EstimatedTokens:   50_000,
		MaxCostPerRequest: 0, // no limit
	})
	if result.Profile.Tier != TierStrong {
		t.Errorf("expected TierStrong with no cost limit, got %s", result.Profile.Tier)
	}
}

// ---------------------------------------------------------------------------
// Concurrency test
// ---------------------------------------------------------------------------

func TestRoute_ConcurrentCallsAreSafe(t *testing.T) {
	r := newTestRouter()
	var wg sync.WaitGroup
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()

			// Mix different operations concurrently.
			switch idx % 5 {
			case 0:
				r.Route(ctx, RoutingRequest{TaskComplexity: "simple"})
			case 1:
				r.Route(ctx, RoutingRequest{TaskComplexity: "complex"})
			case 2:
				r.SetProviderHealth("openai", idx%2 == 0)
			case 3:
				r.RegisterModel(ModelProfile{
					Provider: "test",
					Model:    "concurrent-model",
					Tier:     TierDefault,
				})
			case 4:
				r.AvailableModels(TierDefault)
			}
		}(i)
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Fallback ordering tests
// ---------------------------------------------------------------------------

func TestRoute_FallbackOrderingFromSmall(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "simple",
	})
	if len(result.Fallbacks) < 2 {
		t.Fatalf("expected at least 2 fallbacks, got %d", len(result.Fallbacks))
	}
	// When primary is small, fallback order should be: default tier first, then strong.
	seenDefault := false
	for _, fb := range result.Fallbacks {
		if fb.Tier == TierStrong && !seenDefault {
			t.Error("strong tier appeared before any default tier in fallbacks")
		}
		if fb.Tier == TierDefault {
			seenDefault = true
		}
	}
}

func TestRoute_FallbackOrderingFromStrong(t *testing.T) {
	r := newTestRouter()
	result := r.Route(context.Background(), RoutingRequest{
		TaskComplexity: "complex",
	})
	if len(result.Fallbacks) < 2 {
		t.Fatalf("expected at least 2 fallbacks, got %d", len(result.Fallbacks))
	}
	// When primary is strong, fallback order should be: default tier first, then small.
	seenDefault := false
	for _, fb := range result.Fallbacks {
		if fb.Tier == TierSmall && !seenDefault {
			t.Error("small tier appeared before any default tier in fallbacks")
		}
		if fb.Tier == TierDefault {
			seenDefault = true
		}
	}
}

// ---------------------------------------------------------------------------
// ModelProfile helper tests
// ---------------------------------------------------------------------------

func TestModelProfile_HasCapability(t *testing.T) {
	m := strongModel()
	if !m.hasCapability("vision") {
		t.Error("expected strongModel to have 'vision'")
	}
	if m.hasCapability("quantum") {
		t.Error("expected strongModel NOT to have 'quantum'")
	}
}

func TestModelProfile_HasAllCapabilities(t *testing.T) {
	m := strongModel()
	if !m.hasAllCapabilities([]string{"code", "reasoning"}) {
		t.Error("expected strongModel to have code+reasoning")
	}
	if m.hasAllCapabilities([]string{"code", "quantum"}) {
		t.Error("expected strongModel NOT to have code+quantum")
	}
	// Empty list should always match.
	if !m.hasAllCapabilities(nil) {
		t.Error("nil capabilities should always match")
	}
}

func TestModelProfile_EstimatedRequestCost(t *testing.T) {
	m := smallModel()
	cost := m.estimatedRequestCost(10_000)
	// 10K input: 10 * 0.00015 = 0.0015
	// 2.5K output est: 2.5 * 0.0006 = 0.0015
	// total: 0.003
	expected := 0.003
	if diff := cost - expected; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("expected cost ~%.4f, got %.4f", expected, cost)
	}
}

// ---------------------------------------------------------------------------
// Config defaults tests
// ---------------------------------------------------------------------------

func TestRouterConfig_Defaults(t *testing.T) {
	cfg := RouterConfig{}
	cfg.defaults()
	if cfg.DefaultTier != TierDefault {
		t.Errorf("expected DefaultTier=%s, got %s", TierDefault, cfg.DefaultTier)
	}
	if cfg.CostWeight != 0.3 {
		t.Errorf("expected CostWeight=0.3, got %f", cfg.CostWeight)
	}
	if cfg.LatencyWeight != 0.2 {
		t.Errorf("expected LatencyWeight=0.2, got %f", cfg.LatencyWeight)
	}
}

func TestRouterConfig_DoesNotOverrideExplicit(t *testing.T) {
	cfg := RouterConfig{
		DefaultTier:   TierStrong,
		CostWeight:    0.5,
		LatencyWeight: 0.4,
	}
	cfg.defaults()
	if cfg.DefaultTier != TierStrong {
		t.Errorf("should not override explicit DefaultTier")
	}
	if cfg.CostWeight != 0.5 {
		t.Errorf("should not override explicit CostWeight")
	}
}
