package agent_eval

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBaseline_SaveAndGetBaseline(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	snap := BaselineSnapshot{
		ID:      "snap-001",
		Name:    "v1.0.0-baseline",
		Version: "v1.0.0",
		Metrics: BaselineMetrics{
			SuccessRate:  0.85,
			AvgLatencyMs: 1200,
			P95LatencyMs: 3500,
			AvgTokens:    2000,
			AvgCost:      0.05,
			QualityScore: 0.78,
			TaskCount:    50,
		},
		CreatedAt: time.Now(),
		Tags:      []string{"release", "stable"},
	}

	if err := store.SaveBaseline(ctx, snap); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	loaded, err := store.GetBaseline(ctx, "snap-001")
	if err != nil {
		t.Fatalf("GetBaseline: %v", err)
	}

	if loaded.ID != snap.ID {
		t.Fatalf("expected ID %q, got %q", snap.ID, loaded.ID)
	}
	if loaded.Metrics.SuccessRate != snap.Metrics.SuccessRate {
		t.Fatalf("expected SuccessRate %.2f, got %.2f", snap.Metrics.SuccessRate, loaded.Metrics.SuccessRate)
	}
	if loaded.Metrics.TaskCount != snap.Metrics.TaskCount {
		t.Fatalf("expected TaskCount %d, got %d", snap.Metrics.TaskCount, loaded.Metrics.TaskCount)
	}
	if loaded.Name != snap.Name {
		t.Fatalf("expected Name %q, got %q", snap.Name, loaded.Name)
	}
}

func TestBaseline_GetBaselineNonExistent(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	_, err := store.GetBaseline(ctx, "does-not-exist")
	if err == nil {
		t.Fatalf("expected error for non-existent baseline")
	}
}

func TestBaseline_LatestBaseline(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	first := BaselineSnapshot{
		ID:        "snap-first",
		Name:      "first",
		CreatedAt: time.Now().Add(-time.Hour),
		Metrics:   BaselineMetrics{SuccessRate: 0.70},
	}
	second := BaselineSnapshot{
		ID:        "snap-second",
		Name:      "second",
		CreatedAt: time.Now(),
		Metrics:   BaselineMetrics{SuccessRate: 0.90},
	}

	if err := store.SaveBaseline(ctx, first); err != nil {
		t.Fatalf("SaveBaseline first: %v", err)
	}
	if err := store.SaveBaseline(ctx, second); err != nil {
		t.Fatalf("SaveBaseline second: %v", err)
	}

	latest, err := store.LatestBaseline(ctx)
	if err != nil {
		t.Fatalf("LatestBaseline: %v", err)
	}

	// The latest saved is "second" because SaveBaseline overwrites latest.json.
	if latest.ID != "snap-second" {
		t.Fatalf("expected latest to be snap-second, got %q", latest.ID)
	}
}

func TestBaseline_ListBaselinesReverseChronological(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	base := time.Now()
	snapshots := []BaselineSnapshot{
		{ID: "snap-a", Name: "a", CreatedAt: base.Add(-2 * time.Hour)},
		{ID: "snap-b", Name: "b", CreatedAt: base.Add(-1 * time.Hour)},
		{ID: "snap-c", Name: "c", CreatedAt: base},
	}

	for _, s := range snapshots {
		if err := store.SaveBaseline(ctx, s); err != nil {
			t.Fatalf("SaveBaseline %s: %v", s.ID, err)
		}
	}

	listed, err := store.ListBaselines(ctx, 0)
	if err != nil {
		t.Fatalf("ListBaselines: %v", err)
	}

	if len(listed) != 3 {
		t.Fatalf("expected 3 baselines, got %d", len(listed))
	}

	// Reverse chronological: c, b, a.
	if listed[0].ID != "snap-c" || listed[1].ID != "snap-b" || listed[2].ID != "snap-a" {
		t.Fatalf("unexpected order: %s, %s, %s", listed[0].ID, listed[1].ID, listed[2].ID)
	}
}

func TestBaseline_ListBaselinesRespectsLimit(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	base := time.Now()
	for i := 0; i < 5; i++ {
		snap := BaselineSnapshot{
			ID:        strings.Replace("snap-X", "X", string(rune('a'+i)), 1),
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		if err := store.SaveBaseline(ctx, snap); err != nil {
			t.Fatalf("SaveBaseline %s: %v", snap.ID, err)
		}
	}

	listed, err := store.ListBaselines(ctx, 2)
	if err != nil {
		t.Fatalf("ListBaselines with limit: %v", err)
	}

	if len(listed) != 2 {
		t.Fatalf("expected 2 baselines with limit, got %d", len(listed))
	}
}

func TestBaseline_DeleteBaseline(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	snap := BaselineSnapshot{
		ID:        "snap-delete",
		Name:      "delete-me",
		CreatedAt: time.Now(),
	}

	if err := store.SaveBaseline(ctx, snap); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	if err := store.DeleteBaseline(ctx, "snap-delete"); err != nil {
		t.Fatalf("DeleteBaseline: %v", err)
	}

	_, err := store.GetBaseline(ctx, "snap-delete")
	if err == nil {
		t.Fatalf("expected error after deletion")
	}
}

func TestBaseline_DeleteBaselineUpdatesLatest(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	base := time.Now()
	first := BaselineSnapshot{ID: "snap-older", CreatedAt: base.Add(-time.Hour)}
	second := BaselineSnapshot{ID: "snap-newer", CreatedAt: base}

	if err := store.SaveBaseline(ctx, first); err != nil {
		t.Fatalf("SaveBaseline first: %v", err)
	}
	if err := store.SaveBaseline(ctx, second); err != nil {
		t.Fatalf("SaveBaseline second: %v", err)
	}

	// Delete the latest; latest.json should update to the older one.
	if err := store.DeleteBaseline(ctx, "snap-newer"); err != nil {
		t.Fatalf("DeleteBaseline: %v", err)
	}

	latest, err := store.LatestBaseline(ctx)
	if err != nil {
		t.Fatalf("LatestBaseline after delete: %v", err)
	}
	if latest.ID != "snap-older" {
		t.Fatalf("expected latest to be snap-older after deletion, got %q", latest.ID)
	}
}

// --- DetectRegressions tests ---

func TestBaseline_DetectRegressionsSuccessRateDrop(t *testing.T) {
	baseline := BaselineMetrics{SuccessRate: 0.90, AvgLatencyMs: 1000, QualityScore: 0.80}
	current := BaselineMetrics{SuccessRate: 0.60, AvgLatencyMs: 1000, QualityScore: 0.80}
	cfg := DefaultRegressionConfig()

	alerts := DetectRegressions(baseline, current, cfg)

	found := false
	for _, a := range alerts {
		if a.Metric == "SuccessRate" {
			found = true
			if a.Severity != "critical" {
				t.Fatalf("expected critical severity for ~33%% drop, got %q (delta=%.2f%%)", a.Severity, a.DeltaPercent)
			}
			if a.DeltaPercent < 30 {
				t.Fatalf("expected delta > 30%%, got %.2f%%", a.DeltaPercent)
			}
		}
	}
	if !found {
		t.Fatalf("expected SuccessRate regression alert, got %v", alerts)
	}
}

func TestBaseline_DetectRegressionsLatencyIncrease(t *testing.T) {
	baseline := BaselineMetrics{AvgLatencyMs: 1000, P95LatencyMs: 2000}
	current := BaselineMetrics{AvgLatencyMs: 1500, P95LatencyMs: 3000}
	cfg := DefaultRegressionConfig()

	alerts := DetectRegressions(baseline, current, cfg)

	latencyAlerts := 0
	for _, a := range alerts {
		if a.Metric == "AvgLatencyMs" || a.Metric == "P95LatencyMs" {
			latencyAlerts++
		}
	}
	if latencyAlerts == 0 {
		t.Fatalf("expected latency regression alerts, got %v", alerts)
	}
}

func TestBaseline_DetectRegressionsCostIncrease(t *testing.T) {
	baseline := BaselineMetrics{AvgCost: 0.05}
	current := BaselineMetrics{AvgCost: 0.10}
	cfg := DefaultRegressionConfig()

	alerts := DetectRegressions(baseline, current, cfg)

	found := false
	for _, a := range alerts {
		if a.Metric == "AvgCost" {
			found = true
			if a.DeltaPercent < 90 {
				t.Fatalf("expected ~100%% delta for cost doubling, got %.2f%%", a.DeltaPercent)
			}
		}
	}
	if !found {
		t.Fatalf("expected AvgCost regression alert, got %v", alerts)
	}
}

func TestBaseline_DetectRegressionsRespectsIgnoreMetrics(t *testing.T) {
	baseline := BaselineMetrics{SuccessRate: 0.90, AvgCost: 0.05}
	current := BaselineMetrics{SuccessRate: 0.50, AvgCost: 0.50}
	cfg := RegressionConfig{
		WarningThresholdPercent:  10,
		CriticalThresholdPercent: 25,
		IgnoreMetrics:            []string{"SuccessRate", "AvgCost"},
	}

	alerts := DetectRegressions(baseline, current, cfg)

	for _, a := range alerts {
		if a.Metric == "SuccessRate" || a.Metric == "AvgCost" {
			t.Fatalf("metric %q should have been ignored, got alert: %+v", a.Metric, a)
		}
	}
}

func TestBaseline_DetectRegressionsWarningVsCritical(t *testing.T) {
	baseline := BaselineMetrics{
		SuccessRate:  0.90,
		QualityScore: 0.90,
	}
	cfg := RegressionConfig{
		WarningThresholdPercent:  10,
		CriticalThresholdPercent: 25,
	}

	// ~15% drop in SuccessRate -> warning.
	warningCurrent := BaselineMetrics{SuccessRate: 0.77, QualityScore: 0.90}
	warningAlerts := DetectRegressions(baseline, warningCurrent, cfg)

	foundWarning := false
	for _, a := range warningAlerts {
		if a.Metric == "SuccessRate" {
			foundWarning = true
			if a.Severity != "warning" {
				t.Fatalf("expected warning for ~15%% drop, got %q (delta=%.2f%%)", a.Severity, a.DeltaPercent)
			}
		}
	}
	if !foundWarning {
		t.Fatalf("expected SuccessRate warning alert")
	}

	// ~40% drop in QualityScore -> critical.
	criticalCurrent := BaselineMetrics{SuccessRate: 0.90, QualityScore: 0.50}
	criticalAlerts := DetectRegressions(baseline, criticalCurrent, cfg)

	foundCritical := false
	for _, a := range criticalAlerts {
		if a.Metric == "QualityScore" {
			foundCritical = true
			if a.Severity != "critical" {
				t.Fatalf("expected critical for ~44%% drop, got %q (delta=%.2f%%)", a.Severity, a.DeltaPercent)
			}
		}
	}
	if !foundCritical {
		t.Fatalf("expected QualityScore critical alert")
	}
}

func TestBaseline_DetectRegressionsWithImprovementReturnsNoAlerts(t *testing.T) {
	baseline := BaselineMetrics{
		SuccessRate:  0.70,
		AvgLatencyMs: 2000,
		P95LatencyMs: 4000,
		AvgTokens:    3000,
		AvgCost:      0.10,
		QualityScore: 0.60,
	}
	current := BaselineMetrics{
		SuccessRate:  0.90,
		AvgLatencyMs: 1000,
		P95LatencyMs: 2000,
		AvgTokens:    1500,
		AvgCost:      0.05,
		QualityScore: 0.85,
	}
	cfg := DefaultRegressionConfig()

	alerts := DetectRegressions(baseline, current, cfg)

	if len(alerts) != 0 {
		t.Fatalf("expected no alerts when all metrics improved, got %d: %v", len(alerts), alerts)
	}
}

func TestBaseline_DetectRegressionsIdenticalMetricsReturnsNoAlerts(t *testing.T) {
	metrics := BaselineMetrics{
		SuccessRate:  0.85,
		AvgLatencyMs: 1500,
		P95LatencyMs: 3000,
		AvgTokens:    2500,
		AvgCost:      0.08,
		QualityScore: 0.75,
	}
	cfg := DefaultRegressionConfig()

	alerts := DetectRegressions(metrics, metrics, cfg)

	if len(alerts) != 0 {
		t.Fatalf("expected no alerts for identical metrics, got %d: %v", len(alerts), alerts)
	}
}

// --- FormatRegressionReport tests ---

func TestBaseline_FormatRegressionReportMarkdown(t *testing.T) {
	alerts := []RegressionAlert{
		{
			Metric:       "SuccessRate",
			Baseline:     0.90,
			Current:      0.70,
			DeltaPercent: 22.22,
			Severity:     "critical",
			Message:      "[CRITICAL] SuccessRate regressed by 22.22% (baseline: 0.9, current: 0.7)",
		},
		{
			Metric:       "AvgLatencyMs",
			Baseline:     1000,
			Current:      1200,
			DeltaPercent: 20.00,
			Severity:     "warning",
			Message:      "[WARNING] AvgLatencyMs regressed by 20.00% (baseline: 1000, current: 1200)",
		},
	}

	report := FormatRegressionReport(alerts)

	if !strings.Contains(report, "# Regression Report") {
		t.Fatalf("expected markdown header in report")
	}
	if !strings.Contains(report, "2 regression(s) detected") {
		t.Fatalf("expected regression count in report")
	}
	if !strings.Contains(report, "| SuccessRate") {
		t.Fatalf("expected SuccessRate row in table")
	}
	if !strings.Contains(report, "| AvgLatencyMs") {
		t.Fatalf("expected AvgLatencyMs row in table")
	}
	if !strings.Contains(report, "| Metric |") {
		t.Fatalf("expected table header in report")
	}
	if !strings.Contains(report, "### Details") {
		t.Fatalf("expected details section in report")
	}
}

func TestBaseline_FormatRegressionReportEmpty(t *testing.T) {
	report := FormatRegressionReport(nil)
	if !strings.Contains(report, "No regressions detected") {
		t.Fatalf("expected 'No regressions detected' for empty alerts, got: %s", report)
	}
}

// --- DefaultRegressionConfig tests ---

func TestBaseline_DefaultRegressionConfigValues(t *testing.T) {
	cfg := DefaultRegressionConfig()

	if cfg.WarningThresholdPercent != 10 {
		t.Fatalf("expected WarningThresholdPercent 10, got %.2f", cfg.WarningThresholdPercent)
	}
	if cfg.CriticalThresholdPercent != 25 {
		t.Fatalf("expected CriticalThresholdPercent 25, got %.2f", cfg.CriticalThresholdPercent)
	}
	if len(cfg.IgnoreMetrics) != 0 {
		t.Fatalf("expected no IgnoreMetrics by default, got %v", cfg.IgnoreMetrics)
	}
}

// --- SaveBaseline with auto-generated fields ---

func TestBaseline_SaveBaselineAutoGeneratesIDAndCreatedAt(t *testing.T) {
	dir := t.TempDir()
	store := NewFileBaselineStore(dir)
	ctx := context.Background()

	snap := BaselineSnapshot{
		Name:    "auto-fields",
		Metrics: BaselineMetrics{SuccessRate: 0.80},
	}

	if err := store.SaveBaseline(ctx, snap); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// Should be retrievable via latest since we don't know the auto-generated ID.
	latest, err := store.LatestBaseline(ctx)
	if err != nil {
		t.Fatalf("LatestBaseline: %v", err)
	}

	if latest.ID == "" {
		t.Fatalf("expected auto-generated ID")
	}
	if latest.CreatedAt.IsZero() {
		t.Fatalf("expected auto-generated CreatedAt")
	}
	if latest.Name != "auto-fields" {
		t.Fatalf("expected Name 'auto-fields', got %q", latest.Name)
	}
}

func TestBaseline_BuildBaselineMetrics(t *testing.T) {
	metrics := &EvaluationMetrics{
		Performance: PerformanceMetrics{
			SuccessRate:      0.85,
			AvgExecutionTime: 1200 * time.Millisecond,
			P95Time:          2500 * time.Millisecond,
		},
		Resources: ResourceMetrics{
			AvgTokensUsed:  2100,
			AvgCostPerTask: 0.04,
		},
		Quality: QualityMetrics{
			SolutionQuality: 0.72,
		},
		TotalTasks: 12,
	}

	got, err := BuildBaselineMetrics(metrics)
	if err != nil {
		t.Fatalf("BuildBaselineMetrics: %v", err)
	}
	if got.SuccessRate != metrics.Performance.SuccessRate {
		t.Fatalf("expected SuccessRate %.2f, got %.2f", metrics.Performance.SuccessRate, got.SuccessRate)
	}
	if got.AvgLatencyMs != float64(metrics.Performance.AvgExecutionTime.Milliseconds()) {
		t.Fatalf("expected AvgLatencyMs %.2f, got %.2f", float64(metrics.Performance.AvgExecutionTime.Milliseconds()), got.AvgLatencyMs)
	}
	if got.P95LatencyMs != float64(metrics.Performance.P95Time.Milliseconds()) {
		t.Fatalf("expected P95LatencyMs %.2f, got %.2f", float64(metrics.Performance.P95Time.Milliseconds()), got.P95LatencyMs)
	}
	if got.AvgTokens != metrics.Resources.AvgTokensUsed {
		t.Fatalf("expected AvgTokens %d, got %d", metrics.Resources.AvgTokensUsed, got.AvgTokens)
	}
	if got.AvgCost != metrics.Resources.AvgCostPerTask {
		t.Fatalf("expected AvgCost %.2f, got %.2f", metrics.Resources.AvgCostPerTask, got.AvgCost)
	}
	if got.QualityScore != metrics.Quality.SolutionQuality {
		t.Fatalf("expected QualityScore %.2f, got %.2f", metrics.Quality.SolutionQuality, got.QualityScore)
	}
	if got.TaskCount != metrics.TotalTasks {
		t.Fatalf("expected TaskCount %d, got %d", metrics.TotalTasks, got.TaskCount)
	}
}

func TestBaseline_BuildBaselineSnapshot(t *testing.T) {
	results := &EvaluationResults{
		Metrics: &EvaluationMetrics{
			Performance: PerformanceMetrics{SuccessRate: 0.9},
			Resources:   ResourceMetrics{AvgTokensUsed: 1000, AvgCostPerTask: 0.02},
			Quality:     QualityMetrics{SolutionQuality: 0.8},
			TotalTasks:  5,
		},
	}

	snap, err := BuildBaselineSnapshot(results, "baseline-v1", "v1.0.0", []string{"release"})
	if err != nil {
		t.Fatalf("BuildBaselineSnapshot: %v", err)
	}
	if snap.Name != "baseline-v1" {
		t.Fatalf("expected Name baseline-v1, got %q", snap.Name)
	}
	if snap.Version != "v1.0.0" {
		t.Fatalf("expected Version v1.0.0, got %q", snap.Version)
	}
	if snap.Metrics.TaskCount != 5 {
		t.Fatalf("expected TaskCount 5, got %d", snap.Metrics.TaskCount)
	}
	if len(snap.Tags) != 1 || snap.Tags[0] != "release" {
		t.Fatalf("expected Tags [release], got %v", snap.Tags)
	}
	if snap.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be set")
	}
}
