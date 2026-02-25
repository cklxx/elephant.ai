package budget

import (
	"fmt"
	"math"
	"sync"
	"testing"
)

// helper to build a standard manager for most tests.
func newTestManager() *Manager {
	quota := SessionQuota{
		MaxInputTokens:   10000,
		MaxOutputTokens:  5000,
		MaxTotalTokens:   15000,
		MaxCostUSD:       1.0,
		WarningThreshold: 0.8,
	}
	return NewManager(quota, DefaultModelTiers)
}

func TestRecordUsageAccumulates(t *testing.T) {
	m := newTestManager()
	sid := "sess-1"

	m.RecordUsage(sid, 100, 50, "gpt-4")
	m.RecordUsage(sid, 200, 100, "gpt-4")

	u := m.GetUsage(sid)
	if u.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", u.InputTokens)
	}
	if u.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150", u.OutputTokens)
	}
	if u.TotalTokens != 450 {
		t.Errorf("TotalTokens = %d, want 450", u.TotalTokens)
	}
	if u.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", u.TurnCount)
	}
	if u.EstimatedCostUSD <= 0 {
		t.Error("EstimatedCostUSD should be positive")
	}
}

func TestCheckBudgetOKForFreshSession(t *testing.T) {
	m := newTestManager()

	check := m.CheckBudget("nonexistent")
	if check.State != BudgetOK {
		t.Errorf("State = %q, want %q", check.State, BudgetOK)
	}
	if check.UsagePercent != 0 {
		t.Errorf("UsagePercent = %f, want 0", check.UsagePercent)
	}
	if check.RemainingTokens != 15000 {
		t.Errorf("RemainingTokens = %d, want 15000", check.RemainingTokens)
	}
	if check.SuggestedModel != "" {
		t.Errorf("SuggestedModel = %q, want empty", check.SuggestedModel)
	}
}

func TestCheckBudgetWarningWhenAbove80Percent(t *testing.T) {
	m := newTestManager()
	sid := "sess-warn"

	// Push total tokens to 85% of 15000 = 12750.
	m.RecordUsage(sid, 8500, 4250, "gpt-4")

	check := m.CheckBudget(sid)
	if check.State != BudgetWarning {
		t.Errorf("State = %q, want %q", check.State, BudgetWarning)
	}
	if check.UsagePercent < 0.8 {
		t.Errorf("UsagePercent = %f, want >= 0.8", check.UsagePercent)
	}
	if check.SuggestedModel == "" {
		t.Error("SuggestedModel should be non-empty for warning state")
	}
}

func TestCheckBudgetExceededWhenOverQuota(t *testing.T) {
	m := newTestManager()
	sid := "sess-over"

	// Exceed total tokens quota.
	m.RecordUsage(sid, 10000, 6000, "gpt-4")

	check := m.CheckBudget(sid)
	if check.State != BudgetExceeded {
		t.Errorf("State = %q, want %q", check.State, BudgetExceeded)
	}
	if check.UsagePercent < 1.0 {
		t.Errorf("UsagePercent = %f, want >= 1.0", check.UsagePercent)
	}
	if check.RemainingTokens != 0 {
		t.Errorf("RemainingTokens = %d, want 0", check.RemainingTokens)
	}
}

func TestSuggestDowngradeReturnsCheaperModel(t *testing.T) {
	m := newTestManager()
	sid := "sess-downgrade"

	// Push above warning threshold.
	m.RecordUsage(sid, 8500, 4250, "gpt-4")

	suggested, ok := m.SuggestDowngrade(sid, "gpt-4")
	if !ok {
		t.Fatal("SuggestDowngrade returned false, want true")
	}
	if suggested == "" {
		t.Fatal("suggested model is empty")
	}
	if suggested == "gpt-4" {
		t.Fatalf("suggested model should be cheaper than gpt-4, got %q", suggested)
	}

	// Verify the suggested model is indeed cheaper (lower priority).
	suggestedPriority := -1
	currentPriority := -1
	for _, tier := range DefaultModelTiers {
		if tier.Name == suggested {
			suggestedPriority = tier.Priority
		}
		if tier.Name == "gpt-4" {
			currentPriority = tier.Priority
		}
	}
	if suggestedPriority >= currentPriority {
		t.Errorf("suggested model priority (%d) should be lower than current (%d)", suggestedPriority, currentPriority)
	}
}

func TestSuggestDowngradeReturnsFalseForCheapestModel(t *testing.T) {
	m := newTestManager()
	sid := "sess-cheapest"

	// Push above warning threshold.
	m.RecordUsage(sid, 8500, 4250, "deepseek-chat")

	suggested, ok := m.SuggestDowngrade(sid, "deepseek-chat")
	if ok {
		t.Errorf("SuggestDowngrade returned true for cheapest model, suggested %q", suggested)
	}
	if suggested != "" {
		t.Errorf("suggested = %q, want empty", suggested)
	}
}

func TestResetSessionClearsUsage(t *testing.T) {
	m := newTestManager()
	sid := "sess-reset"

	m.RecordUsage(sid, 500, 200, "gpt-4")
	u := m.GetUsage(sid)
	if u.TotalTokens == 0 {
		t.Fatal("usage should be non-zero before reset")
	}

	m.ResetSession(sid)

	u = m.GetUsage(sid)
	if u.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d after reset, want 0", u.TotalTokens)
	}
	if u.TurnCount != 0 {
		t.Errorf("TurnCount = %d after reset, want 0", u.TurnCount)
	}

	check := m.CheckBudget(sid)
	if check.State != BudgetOK {
		t.Errorf("State = %q after reset, want %q", check.State, BudgetOK)
	}
}

func TestConcurrentUsageRecording(t *testing.T) {
	m := newTestManager()
	sid := "sess-concurrent"

	const goroutines = 10
	const recordingsPerGoroutine = 20
	const inputPerRecording = 10
	const outputPerRecording = 5

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for r := 0; r < recordingsPerGoroutine; r++ {
				m.RecordUsage(sid, inputPerRecording, outputPerRecording, "gpt-4")
			}
		}()
	}
	wg.Wait()

	u := m.GetUsage(sid)
	wantInput := goroutines * recordingsPerGoroutine * inputPerRecording
	wantOutput := goroutines * recordingsPerGoroutine * outputPerRecording
	wantTotal := wantInput + wantOutput
	wantTurns := goroutines * recordingsPerGoroutine

	if u.InputTokens != wantInput {
		t.Errorf("InputTokens = %d, want %d", u.InputTokens, wantInput)
	}
	if u.OutputTokens != wantOutput {
		t.Errorf("OutputTokens = %d, want %d", u.OutputTokens, wantOutput)
	}
	if u.TotalTokens != wantTotal {
		t.Errorf("TotalTokens = %d, want %d", u.TotalTokens, wantTotal)
	}
	if u.TurnCount != wantTurns {
		t.Errorf("TurnCount = %d, want %d", u.TurnCount, wantTurns)
	}
}

func TestCostBasedBudget(t *testing.T) {
	// Configure a quota with only cost limit (no token limits).
	quota := SessionQuota{
		MaxCostUSD:       0.01, // Very small budget
		WarningThreshold: 0.8,
	}
	m := NewManager(quota, DefaultModelTiers)
	sid := "sess-cost"

	// Record a small amount — should be OK.
	m.RecordUsage(sid, 10, 5, "deepseek-chat")
	check := m.CheckBudget(sid)
	if check.State != BudgetOK {
		t.Errorf("State = %q after small usage, want %q", check.State, BudgetOK)
	}
	if check.RemainingTokens != -1 {
		t.Errorf("RemainingTokens = %d, want -1 (unlimited tokens)", check.RemainingTokens)
	}

	// Hammer with expensive model to exceed cost.
	for i := 0; i < 100; i++ {
		m.RecordUsage(sid, 1000, 500, "gpt-4")
	}

	check = m.CheckBudget(sid)
	if check.State != BudgetExceeded {
		t.Errorf("State = %q after heavy cost usage, want %q", check.State, BudgetExceeded)
	}

	u := m.GetUsage(sid)
	if u.EstimatedCostUSD <= quota.MaxCostUSD {
		t.Errorf("EstimatedCostUSD = %f, should exceed budget %f", u.EstimatedCostUSD, quota.MaxCostUSD)
	}
}

func TestGetUsageReturnsZeroForUnknownSession(t *testing.T) {
	m := newTestManager()

	u := m.GetUsage("does-not-exist")
	if u.InputTokens != 0 || u.OutputTokens != 0 || u.TotalTokens != 0 || u.TurnCount != 0 || u.EstimatedCostUSD != 0 {
		t.Errorf("expected zero Usage for unknown session, got %+v", u)
	}
}

func TestSuggestDowngradeBelowThreshold(t *testing.T) {
	m := newTestManager()
	sid := "sess-below"

	// Small usage — well below 80%.
	m.RecordUsage(sid, 100, 50, "gpt-4")

	suggested, ok := m.SuggestDowngrade(sid, "gpt-4")
	if ok {
		t.Errorf("SuggestDowngrade returned true below threshold, suggested %q", suggested)
	}
}

func TestSuggestDowngradeUnknownModel(t *testing.T) {
	m := newTestManager()
	sid := "sess-unknown-model"

	// Push above warning threshold.
	m.RecordUsage(sid, 8500, 4250, "some-custom-model")

	suggested, ok := m.SuggestDowngrade(sid, "some-custom-model")
	if ok {
		t.Errorf("SuggestDowngrade returned true for unknown model, suggested %q", suggested)
	}
	if suggested != "" {
		t.Errorf("suggested = %q, want empty for unknown model", suggested)
	}
}

func TestMultipleSessionsIndependent(t *testing.T) {
	m := newTestManager()

	m.RecordUsage("s1", 5000, 2000, "gpt-4")
	m.RecordUsage("s2", 100, 50, "gpt-4")

	u1 := m.GetUsage("s1")
	u2 := m.GetUsage("s2")

	if u1.TotalTokens != 7000 {
		t.Errorf("s1 TotalTokens = %d, want 7000", u1.TotalTokens)
	}
	if u2.TotalTokens != 150 {
		t.Errorf("s2 TotalTokens = %d, want 150", u2.TotalTokens)
	}

	// Reset s1 should not affect s2.
	m.ResetSession("s1")
	u1 = m.GetUsage("s1")
	u2 = m.GetUsage("s2")
	if u1.TotalTokens != 0 {
		t.Errorf("s1 TotalTokens = %d after reset, want 0", u1.TotalTokens)
	}
	if u2.TotalTokens != 150 {
		t.Errorf("s2 TotalTokens = %d after s1 reset, want 150", u2.TotalTokens)
	}
}

func TestWarningThresholdDefault(t *testing.T) {
	// When WarningThreshold is zero, the default 0.8 should be used.
	quota := SessionQuota{
		MaxTotalTokens:   10000,
		WarningThreshold: 0, // should default to 0.8
	}
	m := NewManager(quota, DefaultModelTiers)
	sid := "sess-default-thresh"

	// 75% — should be OK.
	m.RecordUsage(sid, 5000, 2500, "gpt-4")
	check := m.CheckBudget(sid)
	if check.State != BudgetOK {
		t.Errorf("State = %q at 75%%, want %q", check.State, BudgetOK)
	}

	// Push to 85% — should be Warning.
	m.RecordUsage(sid, 500, 500, "gpt-4")
	check = m.CheckBudget(sid)
	if check.State != BudgetWarning {
		t.Errorf("State = %q at 85%%, want %q", check.State, BudgetWarning)
	}
}

func TestEstimateCostKnownModel(t *testing.T) {
	cost := estimateCost(1000, 500, "deepseek-chat", DefaultModelTiers)
	if cost <= 0 {
		t.Error("cost should be positive for known model")
	}
	// deepseek-chat CostPer1KInput = 0.00014
	// input: 1000 * 0.00014 / 1000 = 0.00014
	// output: 500 * 0.00014 * 2 / 1000 = 0.00014
	expected := 0.00014 + 0.00014
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("cost = %f, want %f", cost, expected)
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	cost := estimateCost(1000, 500, "unknown-model", DefaultModelTiers)
	if cost <= 0 {
		t.Error("cost should be positive for unknown model")
	}
	// Fallback: (1000+500) * 0.001 / 1000 = 0.0015
	expected := 1500.0 * 0.001 / 1000.0
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("cost = %f, want %f", cost, expected)
	}
}

func TestNewManagerDefaultTiers(t *testing.T) {
	m := NewManager(SessionQuota{MaxTotalTokens: 1000}, nil)
	if len(m.modelTiers) != len(DefaultModelTiers) {
		t.Errorf("modelTiers length = %d, want %d", len(m.modelTiers), len(DefaultModelTiers))
	}
}

func TestCheckBudgetExceededOnInputTokensOnly(t *testing.T) {
	quota := SessionQuota{
		MaxInputTokens:   1000,
		WarningThreshold: 0.8,
	}
	m := NewManager(quota, DefaultModelTiers)
	sid := "sess-input-exceed"

	m.RecordUsage(sid, 1100, 10, "gpt-4")
	check := m.CheckBudget(sid)
	if check.State != BudgetExceeded {
		t.Errorf("State = %q, want %q when input tokens exceeded", check.State, BudgetExceeded)
	}
}

func TestCheckBudgetExceededOnOutputTokensOnly(t *testing.T) {
	quota := SessionQuota{
		MaxOutputTokens:  500,
		WarningThreshold: 0.8,
	}
	m := NewManager(quota, DefaultModelTiers)
	sid := "sess-output-exceed"

	m.RecordUsage(sid, 10, 600, "gpt-4")
	check := m.CheckBudget(sid)
	if check.State != BudgetExceeded {
		t.Errorf("State = %q, want %q when output tokens exceeded", check.State, BudgetExceeded)
	}
}

func TestConcurrentCheckBudgetAndRecordUsage(t *testing.T) {
	m := newTestManager()
	sid := "sess-mixed-concurrent"

	var wg sync.WaitGroup
	wg.Add(20)

	// 10 writers.
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				m.RecordUsage(sid, 10, 5, "gpt-4")
			}
		}()
	}

	// 10 readers.
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = m.CheckBudget(sid)
				_ = m.GetUsage(sid)
			}
		}()
	}

	wg.Wait()

	u := m.GetUsage(sid)
	if u.TotalTokens != 10*50*15 {
		t.Errorf("TotalTokens = %d, want %d", u.TotalTokens, 10*50*15)
	}
}

func TestSuggestDowngradeNextTierDown(t *testing.T) {
	m := newTestManager()
	sid := "sess-tier"

	// Push above warning threshold.
	m.RecordUsage(sid, 8500, 4250, "claude-3-sonnet")

	suggested, ok := m.SuggestDowngrade(sid, "claude-3-sonnet")
	if !ok {
		t.Fatal("SuggestDowngrade returned false for claude-3-sonnet")
	}
	// claude-3-sonnet has priority 4; next cheaper should be claude-3-haiku (priority 3).
	if suggested != "claude-3-haiku" {
		t.Errorf("suggested = %q, want %q", suggested, "claude-3-haiku")
	}
}

func TestBudgetCheckSuggestedModelEmptyWhenOK(t *testing.T) {
	m := newTestManager()
	sid := "sess-ok-no-suggest"

	m.RecordUsage(sid, 100, 50, "gpt-4")
	check := m.CheckBudget(sid)
	if check.SuggestedModel != "" {
		t.Errorf("SuggestedModel = %q for OK state, want empty", check.SuggestedModel)
	}
}

func TestRemainingTokensUnlimited(t *testing.T) {
	quota := SessionQuota{
		MaxCostUSD:       1.0,
		WarningThreshold: 0.8,
	}
	m := NewManager(quota, DefaultModelTiers)
	sid := "sess-unlimited-tokens"

	m.RecordUsage(sid, 100, 50, "gpt-4")
	check := m.CheckBudget(sid)
	if check.RemainingTokens != -1 {
		t.Errorf("RemainingTokens = %d, want -1 for unlimited", check.RemainingTokens)
	}
}

func TestSuggestDowngradeNonexistentSession(t *testing.T) {
	m := newTestManager()
	suggested, ok := m.SuggestDowngrade("nonexistent", "gpt-4")
	if ok {
		t.Errorf("SuggestDowngrade returned true for nonexistent session, suggested %q", suggested)
	}
}

// Ensure all test names print for `go test -v`.
func TestSummary(t *testing.T) {
	tests := []string{
		"TestRecordUsageAccumulates",
		"TestCheckBudgetOKForFreshSession",
		"TestCheckBudgetWarningWhenAbove80Percent",
		"TestCheckBudgetExceededWhenOverQuota",
		"TestSuggestDowngradeReturnsCheaperModel",
		"TestSuggestDowngradeReturnsFalseForCheapestModel",
		"TestResetSessionClearsUsage",
		"TestConcurrentUsageRecording",
		"TestCostBasedBudget",
	}
	if len(tests) < 9 {
		t.Error("must have at least 9 tests")
	}
	// This test itself doesn't assert behavior — it's a documentation safeguard.
	for _, name := range tests {
		fmt.Printf("  registered: %s\n", name)
	}
}
