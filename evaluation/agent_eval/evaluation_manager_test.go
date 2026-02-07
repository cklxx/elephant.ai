package agent_eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/evaluation/swe_bench"
)

func TestScoreResultsAndFallback(t *testing.T) {
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: t.TempDir()})

	results := []swe_bench.WorkerResult{
		{
			TaskID:     "task-1",
			InstanceID: "inst-1",
			Status:     swe_bench.StatusCompleted,
			Duration:   45 * time.Second,
		},
		{
			TaskID:     "task-2",
			InstanceID: "inst-2",
			Status:     swe_bench.StatusFailed,
			Duration:   2 * time.Minute,
			Error:      "boom",
		},
	}

	scores := em.scoreResults(results)
	if len(scores) != len(results) {
		t.Fatalf("expected %d scores, got %d", len(results), len(scores))
	}
	if scores[0].Score <= scores[1].Score {
		t.Fatalf("completed task should score higher: %+v", scores)
	}

	analysis := em.fallbackAnalysis(results, scores)
	if analysis == nil {
		t.Fatalf("fallback analysis should be produced when metrics are missing")
	}
	if analysis.Summary.OverallScore == 0 {
		t.Fatalf("overall score should be populated: %+v", analysis.Summary)
	}

	profile := em.buildAgentProfile("agent-123", nil, scores, results)
	if profile == nil {
		t.Fatalf("profile should be generated even without metrics")
	}
	if profile.AvgSuccessRate <= 0 {
		t.Fatalf("profile success rate should reflect results: %+v", profile)
	}
}

func TestAgentStoreExposure(t *testing.T) {
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: t.TempDir()})

	profile := &AgentProfile{AgentID: "agent-99", EvaluationCount: 1}
	if _, err := em.agentStore.UpsertProfile(profile); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	loaded, err := em.GetAgentProfile("agent-99")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if loaded == nil || loaded.AgentID != "agent-99" {
		t.Fatalf("unexpected profile: %+v", loaded)
	}

	eval := &EvaluationResults{JobID: "job-xyz", Config: &EvaluationConfig{DatasetPath: "bar"}}
	if err := em.agentStore.StoreEvaluation("agent-99", eval); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	history, err := em.ListAgentEvaluations("agent-99")
	if err != nil {
		t.Fatalf("list evaluations: %v", err)
	}
	if len(history) != 1 || history[0].JobID != "job-xyz" {
		t.Fatalf("unexpected history: %+v", history)
	}
}

func TestHydrateFromStore(t *testing.T) {
	dir := t.TempDir()
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: dir, AgentID: "agent-restore"})

	stored := &EvaluationResults{JobID: "job-restore", AgentID: "agent-restore", Timestamp: time.Now()}
	if err := em.agentStore.StoreEvaluation("agent-restore", stored); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	if err := em.HydrateFromStore(); err != nil {
		t.Fatalf("hydrate from store: %v", err)
	}

	job, err := em.GetJob("job-restore")
	if err != nil {
		t.Fatalf("expected job to be restored: %v", err)
	}
	if job.Results == nil || job.Results.JobID != "job-restore" {
		t.Fatalf("restored job missing results: %+v", job.Results)
	}
}

func TestGetJobResultsRestoresFromStore(t *testing.T) {
	dir := t.TempDir()
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: dir, AgentID: "agent-123"})

	stored := &EvaluationResults{
		JobID:     "job-123",
		AgentID:   "agent-123",
		Timestamp: time.Now(),
		Config:    &EvaluationConfig{DatasetPath: "foo"},
		Results: []swe_bench.WorkerResult{
			{TaskID: "task-1", InstanceID: "inst-1", Status: swe_bench.StatusCompleted},
		},
		AutoScores: []AutoScore{{TaskID: "task-1", InstanceID: "inst-1", Score: 90, Grade: "A"}},
		Analysis:   &AnalysisResult{Summary: AnalysisSummary{OverallScore: 0.9, PerformanceGrade: "A"}},
	}

	if err := em.agentStore.StoreEvaluation("agent-123", stored); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	results, err := em.GetJobResults("job-123")
	if err != nil {
		t.Fatalf("expected stored evaluation to be loaded: %v", err)
	}
	if results.JobID != stored.JobID || results.AgentID != stored.AgentID {
		t.Fatalf("unexpected results: %+v", results)
	}
	if status := em.GetJobStatus("job-123"); status != JobStatusCompleted {
		t.Fatalf("expected job status to be completed after restore, got %s", status)
	}
}

func TestDeleteEvaluationRemovesPersistedState(t *testing.T) {
	dir := t.TempDir()
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: dir, AgentID: "agent-del"})

	stored := &EvaluationResults{JobID: "job-del", AgentID: "agent-del", Timestamp: time.Now()}
	if err := em.agentStore.StoreEvaluation("agent-del", stored); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	if err := em.HydrateFromStore(); err != nil {
		t.Fatalf("hydrate from store: %v", err)
	}

	if err := em.DeleteEvaluation("job-del"); err != nil {
		t.Fatalf("delete evaluation: %v", err)
	}

	if _, err := em.agentStore.LoadEvaluation("job-del"); !os.IsNotExist(err) {
		t.Fatalf("expected evaluation to be removed from disk, got %v", err)
	}

	if _, err := em.GetJob("job-del"); err == nil {
		t.Fatalf("expected job to be evicted from in-memory index")
	}
}

func TestQueryEvaluationsFiltersThroughManager(t *testing.T) {
	dir := t.TempDir()
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: dir})

	now := time.Now()
	toStore := []*EvaluationResults{
		{JobID: "job-a", AgentID: "agent-a", Timestamp: now.Add(-2 * time.Minute), Analysis: &AnalysisResult{Summary: AnalysisSummary{OverallScore: 0.5}}},
		{JobID: "job-b", AgentID: "agent-a", Timestamp: now.Add(-30 * time.Second), Analysis: &AnalysisResult{Summary: AnalysisSummary{OverallScore: 0.9}}},
		{JobID: "job-c", AgentID: "agent-b", Timestamp: now.Add(-1 * time.Minute), Analysis: &AnalysisResult{Summary: AnalysisSummary{OverallScore: 0.8}}},
	}

	for _, eval := range toStore {
		if err := em.agentStore.StoreEvaluation(eval.AgentID, eval); err != nil {
			t.Fatalf("store evaluation %s: %v", eval.JobID, err)
		}
	}

	query := EvaluationQuery{AgentID: "agent-a", MinScore: 0.8}
	results, err := em.QueryEvaluations(query)
	if err != nil {
		t.Fatalf("query evaluations: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].JobID != "job-b" {
		t.Fatalf("expected job-b to satisfy filter, got %s", results[0].JobID)
	}

	limited, err := em.QueryEvaluations(EvaluationQuery{Limit: 2})
	if err != nil {
		t.Fatalf("query evaluations with limit: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("expected 2 results with limit applied, got %d", len(limited))
	}
}

func TestQueryEvaluationsFiltersByDatasetThroughManager(t *testing.T) {
	dir := t.TempDir()
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: dir})

	now := time.Now()
	sweBench := &EvaluationResults{JobID: "job-swe", AgentID: "agent-a", Timestamp: now.Add(-30 * time.Second), Config: &EvaluationConfig{DatasetPath: "./data/swe_bench/dev.json", DatasetType: "swe_bench"}, Analysis: &AnalysisResult{Summary: AnalysisSummary{OverallScore: 0.9}}}
	custom := &EvaluationResults{JobID: "job-custom", AgentID: "agent-b", Timestamp: now.Add(-1 * time.Minute), Config: &EvaluationConfig{DatasetPath: "./data/custom.json", DatasetType: "custom"}, Analysis: &AnalysisResult{Summary: AnalysisSummary{OverallScore: 0.8}}}

	for _, eval := range []*EvaluationResults{sweBench, custom} {
		if err := em.agentStore.StoreEvaluation(eval.AgentID, eval); err != nil {
			t.Fatalf("store evaluation %s: %v", eval.JobID, err)
		}
	}

	results, err := em.QueryEvaluations(EvaluationQuery{DatasetType: "swe_bench"})
	if err != nil {
		t.Fatalf("query evaluations: %v", err)
	}

	if len(results) != 1 || results[0].JobID != "job-swe" {
		t.Fatalf("expected only swe_bench evaluation to be returned, got %v", results)
	}
}

func TestQueryEvaluationsFiltersByTagsThroughManager(t *testing.T) {
	dir := t.TempDir()
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: dir})

	if _, err := em.agentStore.UpsertProfile(&AgentProfile{AgentID: "agent-a", Tags: []string{"bench", "nightly"}}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}
	if _, err := em.agentStore.UpsertProfile(&AgentProfile{AgentID: "agent-b", Tags: []string{"staging"}}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	now := time.Now()
	target := &EvaluationResults{JobID: "job-bench", AgentID: "agent-a", Timestamp: now.Add(-10 * time.Second)}
	other := &EvaluationResults{JobID: "job-other", AgentID: "agent-b", Timestamp: now.Add(-5 * time.Second)}

	for _, eval := range []*EvaluationResults{target, other} {
		if err := em.agentStore.StoreEvaluation(eval.AgentID, eval); err != nil {
			t.Fatalf("store evaluation %s: %v", eval.JobID, err)
		}
	}

	results, err := em.QueryEvaluations(EvaluationQuery{Tags: []string{"bench"}})
	if err != nil {
		t.Fatalf("query evaluations: %v", err)
	}

	if len(results) != 1 || results[0].JobID != "job-bench" {
		t.Fatalf("expected only tagged evaluation to be returned, got %v", results)
	}
}

func TestGenerateReportReturnsNormalizedArtifactPath(t *testing.T) {
	dir := t.TempDir()
	em := NewEvaluationManager(&EvaluationConfig{OutputDir: dir})

	metrics := &EvaluationMetrics{
		TotalTasks: 1,
		Performance: PerformanceMetrics{
			SuccessRate:      1,
			AvgExecutionTime: time.Second,
			MedianTime:       time.Second,
			P95Time:          time.Second,
		},
		Quality: QualityMetrics{
			SolutionQuality:    0.9,
			ErrorRecoveryRate:  1,
			ConsistencyScore:   0.9,
			ComplexityHandling: 0.9,
		},
		Resources: ResourceMetrics{
			AvgTokensUsed:  10,
			TotalTokens:    10,
			AvgCostPerTask: 0.01,
			TotalCost:      0.01,
			MemoryUsage:    16,
		},
		Behavior: BehaviorMetrics{
			AvgToolCalls:     1,
			ToolUsagePattern: map[string]int{"bash": 1},
			CommonFailures:   map[string]int{},
		},
	}

	evaluation := &EvaluationResults{
		JobID:   "job-report",
		Metrics: metrics,
		Analysis: &AnalysisResult{
			Summary: AnalysisSummary{
				OverallScore:     0.9,
				PerformanceGrade: "A",
				RiskLevel:        "low",
			},
		},
		Results:   []swe_bench.WorkerResult{{TaskID: "task-1", Status: swe_bench.StatusCompleted}},
		Timestamp: time.Now(),
	}

	reportPath, err := em.generateReport(context.Background(), evaluation, &EvaluationConfig{
		OutputDir:    dir,
		ReportFormat: "markdown",
	})
	if err != nil {
		t.Fatalf("generate report: %v", err)
	}
	if !filepath.IsAbs(reportPath) {
		t.Fatalf("expected absolute report path, got %q", reportPath)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected report artifact to exist: %v", err)
	}
}
