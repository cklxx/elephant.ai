package tools

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// newTestRouter creates an SLARouter backed by a fresh collector and default config.
func newTestRouter(t *testing.T) (*SLARouter, *SLACollector) {
	t.Helper()
	reg := prometheus.NewRegistry()
	collector := NewSLACollector(reg)
	router := NewSLARouter(collector, DefaultSLARouterConfig())
	return router, collector
}

// --- ComputeHealthScore tests -----------------------------------------------

func TestSLARouter_ComputeHealthScore_PerfectMetrics(t *testing.T) {
	router, _ := newTestRouter(t)

	sla := ToolSLA{
		ToolName:    "perfect",
		P95Latency:  50 * time.Millisecond, // very fast
		ErrorRate:   0.0,
		SuccessRate: 1.0,
		CallCount:   100,
	}
	score := router.ComputeHealthScore(sla)
	if math.Abs(score-1.0) > 0.02 {
		t.Errorf("perfect metrics: expected score ~1.0, got %f", score)
	}
}

func TestSLARouter_ComputeHealthScore_BadLatency(t *testing.T) {
	router, _ := newTestRouter(t)

	sla := ToolSLA{
		ToolName:    "slow",
		P95Latency:  10 * time.Second, // 10000ms >> 5000ms threshold
		ErrorRate:   0.0,
		SuccessRate: 1.0,
		CallCount:   100,
	}
	score := router.ComputeHealthScore(sla)
	// latencyScore = 0.0 (capped), errorScore = 1.0, reliabilityScore = 1.0
	// health = 0.4*0 + 0.4*1 + 0.2*1 = 0.6
	expected := 0.6
	if math.Abs(score-expected) > 0.02 {
		t.Errorf("bad latency: expected score ~%f, got %f", expected, score)
	}
}

func TestSLARouter_ComputeHealthScore_HighErrorRate(t *testing.T) {
	router, _ := newTestRouter(t)

	sla := ToolSLA{
		ToolName:    "errors",
		P95Latency:  50 * time.Millisecond,
		ErrorRate:   0.6, // double the max
		SuccessRate: 0.4,
		CallCount:   100,
	}
	score := router.ComputeHealthScore(sla)
	// latencyScore ~1.0, errorScore = 0.0 (0.6/0.3 >= 1.0), reliabilityScore = 0.4/0.7 ~0.571
	// health = 0.4*1 + 0.4*0 + 0.2*0.571 = 0.514
	expected := 0.4*1.0 + 0.4*0.0 + 0.2*(0.4/0.7)
	if math.Abs(score-expected) > 0.02 {
		t.Errorf("high error rate: expected score ~%f, got %f", expected, score)
	}
}

func TestSLARouter_ComputeHealthScore_LowSuccessRate(t *testing.T) {
	router, _ := newTestRouter(t)

	sla := ToolSLA{
		ToolName:    "unreliable",
		P95Latency:  50 * time.Millisecond,
		ErrorRate:   0.0,
		SuccessRate: 0.3, // well below 0.7 threshold
		CallCount:   100,
	}
	score := router.ComputeHealthScore(sla)
	// latencyScore ~1.0, errorScore = 1.0, reliabilityScore = 0.3/0.7 ~0.429
	// health = 0.4*1 + 0.4*1 + 0.2*0.429 = 0.886
	expected := 0.4*1.0 + 0.4*1.0 + 0.2*(0.3/0.7)
	if math.Abs(score-expected) > 0.02 {
		t.Errorf("low success rate: expected score ~%f, got %f", expected, score)
	}
}

func TestSLARouter_ComputeHealthScore_InsufficientCalls(t *testing.T) {
	router, _ := newTestRouter(t)

	sla := ToolSLA{
		ToolName:    "new_tool",
		P95Latency:  10 * time.Second,
		ErrorRate:   1.0,
		SuccessRate: 0.0,
		CallCount:   5, // below MinCallCount of 10
	}
	score := router.ComputeHealthScore(sla)
	if score != 1.0 {
		t.Errorf("insufficient calls: expected score 1.0, got %f", score)
	}
}

func TestSLARouter_ComputeHealthScore_AllBad(t *testing.T) {
	router, _ := newTestRouter(t)

	sla := ToolSLA{
		ToolName:    "terrible",
		P95Latency:  20 * time.Second,
		ErrorRate:   1.0,
		SuccessRate: 0.0,
		CallCount:   100,
	}
	score := router.ComputeHealthScore(sla)
	// latencyScore = 0, errorScore = 0, reliabilityScore = 0
	// health = 0
	if score != 0.0 {
		t.Errorf("all bad: expected score 0.0, got %f", score)
	}
}

// --- GetProfile tests -------------------------------------------------------

func TestSLARouter_GetProfile_ReturnsComputedHealthScore(t *testing.T) {
	router, collector := newTestRouter(t)

	// Record 20 calls to exceed MinCallCount (10), all successful, fast.
	for i := 0; i < 20; i++ {
		collector.RecordExecution("profile_tool", 50*time.Millisecond, nil)
	}

	profile := router.GetProfile("profile_tool")
	if profile.ToolName != "profile_tool" {
		t.Errorf("expected ToolName='profile_tool', got %q", profile.ToolName)
	}
	if profile.SLA.CallCount != 20 {
		t.Errorf("expected CallCount=20, got %d", profile.SLA.CallCount)
	}
	if profile.HealthScore < 0.9 {
		t.Errorf("expected high health score for healthy tool, got %f", profile.HealthScore)
	}
	if !profile.Recommended {
		t.Error("expected healthy tool to be recommended")
	}
}

func TestSLARouter_GetProfile_UnknownTool(t *testing.T) {
	router, _ := newTestRouter(t)

	profile := router.GetProfile("unknown_tool")
	if profile.ToolName != "unknown_tool" {
		t.Errorf("expected ToolName='unknown_tool', got %q", profile.ToolName)
	}
	// No calls → assumed healthy
	if profile.HealthScore != 1.0 {
		t.Errorf("expected HealthScore=1.0 for unknown tool, got %f", profile.HealthScore)
	}
	if !profile.Recommended {
		t.Error("expected unknown tool to be recommended (assumed healthy)")
	}
}

// --- RankTools tests --------------------------------------------------------

func TestSLARouter_RankTools_SortsByHealthScoreDescending(t *testing.T) {
	router, collector := newTestRouter(t)

	// tool_good: 20 successful calls, fast
	for i := 0; i < 20; i++ {
		collector.RecordExecution("tool_good", 50*time.Millisecond, nil)
	}

	// tool_bad: 20 calls, all errors, slow
	for i := 0; i < 20; i++ {
		collector.RecordExecution("tool_bad", 8*time.Second, fmt.Errorf("fail"))
	}

	// tool_medium: 20 calls, half errors
	for i := 0; i < 10; i++ {
		collector.RecordExecution("tool_medium", 2*time.Second, nil)
	}
	for i := 0; i < 10; i++ {
		collector.RecordExecution("tool_medium", 2*time.Second, fmt.Errorf("fail"))
	}

	ranked := router.RankTools([]string{"tool_bad", "tool_medium", "tool_good"})
	if len(ranked) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(ranked))
	}
	if ranked[0].ToolName != "tool_good" {
		t.Errorf("expected best tool first, got %q", ranked[0].ToolName)
	}
	if ranked[1].ToolName != "tool_medium" {
		t.Errorf("expected medium tool second, got %q", ranked[1].ToolName)
	}
	if ranked[2].ToolName != "tool_bad" {
		t.Errorf("expected worst tool last, got %q", ranked[2].ToolName)
	}

	// Verify descending order
	for i := 0; i < len(ranked)-1; i++ {
		if ranked[i].HealthScore < ranked[i+1].HealthScore {
			t.Errorf("rank %d (score %.3f) < rank %d (score %.3f)", i, ranked[i].HealthScore, i+1, ranked[i+1].HealthScore)
		}
	}
}

func TestSLARouter_RankTools_EmptyList(t *testing.T) {
	router, _ := newTestRouter(t)
	ranked := router.RankTools(nil)
	if len(ranked) != 0 {
		t.Errorf("expected empty result, got %d profiles", len(ranked))
	}
}

// --- SelectBest tests -------------------------------------------------------

func TestSLARouter_SelectBest_ReturnsHighestScoring(t *testing.T) {
	router, collector := newTestRouter(t)

	// tool_fast: healthy
	for i := 0; i < 20; i++ {
		collector.RecordExecution("tool_fast", 50*time.Millisecond, nil)
	}

	// tool_slow: degraded
	for i := 0; i < 20; i++ {
		collector.RecordExecution("tool_slow", 8*time.Second, fmt.Errorf("fail"))
	}

	best, ok := router.SelectBest([]string{"tool_slow", "tool_fast"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if best != "tool_fast" {
		t.Errorf("expected best='tool_fast', got %q", best)
	}
}

func TestSLARouter_SelectBest_EmptyReturnsNotOK(t *testing.T) {
	router, _ := newTestRouter(t)

	best, ok := router.SelectBest(nil)
	if ok {
		t.Error("expected ok=false for empty tool list")
	}
	if best != "" {
		t.Errorf("expected empty string, got %q", best)
	}

	best2, ok2 := router.SelectBest([]string{})
	if ok2 {
		t.Error("expected ok=false for empty slice")
	}
	if best2 != "" {
		t.Errorf("expected empty string, got %q", best2)
	}
}

func TestSLARouter_SelectBest_SingleTool(t *testing.T) {
	router, _ := newTestRouter(t)

	best, ok := router.SelectBest([]string{"only_tool"})
	if !ok {
		t.Fatal("expected ok=true for single tool")
	}
	if best != "only_tool" {
		t.Errorf("expected 'only_tool', got %q", best)
	}
}

// --- IsHealthy tests --------------------------------------------------------

func TestSLARouter_IsHealthy_HealthyTool(t *testing.T) {
	router, collector := newTestRouter(t)

	for i := 0; i < 20; i++ {
		collector.RecordExecution("healthy_tool", 50*time.Millisecond, nil)
	}

	if !router.IsHealthy("healthy_tool") {
		t.Error("expected healthy tool to be healthy")
	}
}

func TestSLARouter_IsHealthy_DegradedTool(t *testing.T) {
	router, collector := newTestRouter(t)

	// All errors, very slow → health score should be well below 0.7
	for i := 0; i < 20; i++ {
		collector.RecordExecution("degraded_tool", 8*time.Second, fmt.Errorf("fail"))
	}

	if router.IsHealthy("degraded_tool") {
		profile := router.GetProfile("degraded_tool")
		t.Errorf("expected degraded tool to be unhealthy, got score=%f", profile.HealthScore)
	}
}

func TestSLARouter_IsHealthy_UnknownTool(t *testing.T) {
	router, _ := newTestRouter(t)

	// Unknown tool should be assumed healthy (insufficient data)
	if !router.IsHealthy("never_called") {
		t.Error("expected unknown tool to be considered healthy")
	}
}

// --- DefaultSLARouterConfig tests -------------------------------------------

func TestSLARouter_DefaultSLARouterConfig_SensibleValues(t *testing.T) {
	cfg := DefaultSLARouterConfig()

	if cfg.MaxP95LatencyMs != 5000 {
		t.Errorf("MaxP95LatencyMs = %f, want 5000", cfg.MaxP95LatencyMs)
	}
	if cfg.MaxErrorRate != 0.3 {
		t.Errorf("MaxErrorRate = %f, want 0.3", cfg.MaxErrorRate)
	}
	if cfg.MinSuccessRate != 0.7 {
		t.Errorf("MinSuccessRate = %f, want 0.7", cfg.MinSuccessRate)
	}
	if cfg.MinCallCount != 10 {
		t.Errorf("MinCallCount = %d, want 10", cfg.MinCallCount)
	}
	if cfg.LatencyWeight != 0.4 {
		t.Errorf("LatencyWeight = %f, want 0.4", cfg.LatencyWeight)
	}
	if cfg.ErrorWeight != 0.4 {
		t.Errorf("ErrorWeight = %f, want 0.4", cfg.ErrorWeight)
	}
	if cfg.ReliabilityWeight != 0.2 {
		t.Errorf("ReliabilityWeight = %f, want 0.2", cfg.ReliabilityWeight)
	}

	// Weights should sum to 1.0
	totalWeight := cfg.LatencyWeight + cfg.ErrorWeight + cfg.ReliabilityWeight
	if math.Abs(totalWeight-1.0) > 0.001 {
		t.Errorf("weights sum = %f, want 1.0", totalWeight)
	}
}

// --- Concurrent access safety -----------------------------------------------

func TestSLARouter_ConcurrentAccess(t *testing.T) {
	router, collector := newTestRouter(t)

	const goroutines = 20
	const opsPerGoroutine = 50
	toolNames := []string{"conc_a", "conc_b", "conc_c"}

	var wg sync.WaitGroup

	// Writers: record metrics concurrently.
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			name := toolNames[id%len(toolNames)]
			for i := 0; i < opsPerGoroutine; i++ {
				var err error
				if i%5 == 0 {
					err = fmt.Errorf("transient")
				}
				collector.RecordExecution(name, time.Duration(i)*time.Millisecond, err)
			}
		}(g)
	}

	// Readers: query router concurrently.
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				_ = router.GetProfile(toolNames[i%len(toolNames)])
				_ = router.RankTools(toolNames)
				_, _ = router.SelectBest(toolNames)
				_ = router.IsHealthy(toolNames[i%len(toolNames)])
			}
		}()
	}

	wg.Wait()

	// Verify data integrity: all tools should have metrics
	for _, name := range toolNames {
		profile := router.GetProfile(name)
		if profile.SLA.CallCount == 0 {
			t.Errorf("expected calls for %q after concurrent writes", name)
		}
		if profile.HealthScore < 0 || profile.HealthScore > 1 {
			t.Errorf("health score for %q out of range: %f", name, profile.HealthScore)
		}
	}
}
