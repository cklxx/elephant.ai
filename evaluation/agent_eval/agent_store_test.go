package agent_eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentDataStoreUpsertAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	base := &AgentProfile{
		AgentID:         "agent-1",
		AvgSuccessRate:  0.8,
		AvgExecTime:     time.Second,
		AvgCostPerTask:  0.1,
		AvgQualityScore: 0.7,
		EvaluationCount: 1,
	}

	saved, err := store.UpsertProfile(base)
	if err != nil {
		t.Fatalf("upsert base profile: %v", err)
	}
	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should be populated: %+v", saved)
	}

	// Second update should merge with weighted averages.
	next := &AgentProfile{
		AgentID:         "agent-1",
		AvgSuccessRate:  0.6,
		AvgExecTime:     2 * time.Second,
		AvgCostPerTask:  0.2,
		AvgQualityScore: 0.5,
		EvaluationCount: 1,
	}

	merged, err := store.UpsertProfile(next)
	if err != nil {
		t.Fatalf("upsert merged profile: %v", err)
	}

	if merged.EvaluationCount != 2 {
		t.Fatalf("expected evaluation count to accumulate, got %d", merged.EvaluationCount)
	}

	// Weighted averages between 0.8 and 0.6, and 1s and 2s respectively.
	if merged.AvgSuccessRate < 0.69 || merged.AvgSuccessRate > 0.71 {
		t.Fatalf("unexpected weighted success rate: %.3f", merged.AvgSuccessRate)
	}
	if merged.AvgExecTime < 1500*time.Millisecond || merged.AvgExecTime > 1600*time.Millisecond {
		t.Fatalf("unexpected weighted execution time: %s", merged.AvgExecTime)
	}

	loaded, err := store.LoadProfile("agent-1")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}

	if loaded.AgentID != merged.AgentID || loaded.AvgQualityScore == 0 {
		t.Fatalf("loaded profile mismatch: %+v", loaded)
	}
}

func TestStoreEvaluationWritesFile(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	results := &EvaluationResults{JobID: "job-123"}
	if err := store.StoreEvaluation("agent-xyz", results); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	if results.Timestamp.IsZero() {
		t.Fatalf("expected timestamp to be backfilled")
	}

	expected := filepath.Join(dir, "agent-xyz", "job-123.json")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected evaluation file to exist: %v", err)
	}
}

func TestStoreEvaluationRequiresJobID(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	if err := store.StoreEvaluation("agent-xyz", &EvaluationResults{}); err == nil {
		t.Fatalf("expected error when job id missing")
	}
}

func TestListProfilesAndEvaluations(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	profiles := []*AgentProfile{{AgentID: "agent-a", EvaluationCount: 1}, {AgentID: "agent-b", EvaluationCount: 2}}
	for _, p := range profiles {
		if _, err := store.UpsertProfile(p); err != nil {
			t.Fatalf("upsert profile %s: %v", p.AgentID, err)
		}
	}

	listed, err := store.ListProfiles()
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(listed) != len(profiles) {
		t.Fatalf("expected %d profiles, got %d", len(profiles), len(listed))
	}

	snapshot := &EvaluationResults{JobID: "job-1", Config: &EvaluationConfig{DatasetPath: "foo.json"}}
	if err := store.StoreEvaluation("agent-a", snapshot); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	evals, err := store.ListEvaluations("agent-a")
	if err != nil {
		t.Fatalf("list evaluations: %v", err)
	}
	if len(evals) != 1 || evals[0].JobID != "job-1" {
		t.Fatalf("unexpected evaluations: %+v", evals)
	}
	if evals[0].Config == nil || evals[0].Config.DatasetPath != "foo.json" {
		t.Fatalf("expected evaluation config to round-trip: %+v", evals[0].Config)
	}
}

func TestLoadEvaluationByJobID(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	snapshot := &EvaluationResults{JobID: "job-123", Timestamp: time.Now()}
	if err := store.StoreEvaluation("agent-xyz", snapshot); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	loaded, err := store.LoadEvaluation("job-123")
	if err != nil {
		t.Fatalf("load evaluation: %v", err)
	}
	if loaded.AgentID != "agent-xyz" {
		t.Fatalf("expected agent id to be persisted in snapshot: %+v", loaded)
	}

	// Ensure the index is used on subsequent lookups.
	again, err := store.LoadEvaluation("job-123")
	if err != nil {
		t.Fatalf("load evaluation from index: %v", err)
	}
	if again.JobID != "job-123" || again.AgentID != "agent-xyz" {
		t.Fatalf("unexpected evaluation loaded from index: %+v", again)
	}
}

func TestAgentDataStoreConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	profiles := []*AgentProfile{{AgentID: "agent-1"}, {AgentID: "agent-2"}}
	done := make(chan struct{}, len(profiles))

	for _, p := range profiles {
		p := p
		go func() {
			if _, err := store.UpsertProfile(&AgentProfile{AgentID: p.AgentID, EvaluationCount: 1}); err != nil {
				t.Errorf("upsert profile %s: %v", p.AgentID, err)
			}
			snapshot := &EvaluationResults{JobID: p.AgentID + "-job", Timestamp: time.Now()}
			if err := store.StoreEvaluation(p.AgentID, snapshot); err != nil {
				t.Errorf("store evaluation %s: %v", snapshot.JobID, err)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < len(profiles); i++ {
		<-done
	}

	listed, err := store.ListProfiles()
	if err != nil {
		t.Fatalf("list profiles after concurrent writes: %v", err)
	}
	if len(listed) != len(profiles) {
		t.Fatalf("expected %d profiles, got %d", len(profiles), len(listed))
	}

	evals, err := store.ListAllEvaluations()
	if err != nil {
		t.Fatalf("list evaluations after concurrent writes: %v", err)
	}
	if len(evals) != len(profiles) {
		t.Fatalf("expected %d evaluations, got %d", len(profiles), len(evals))
	}
}

func TestRemoveEvaluationDeletesSnapshotAndIndex(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	snapshot := &EvaluationResults{JobID: "job-remove", Timestamp: time.Now()}
	if err := store.StoreEvaluation("agent-remove", snapshot); err != nil {
		t.Fatalf("store evaluation: %v", err)
	}

	if err := store.RemoveEvaluation("job-remove"); err != nil {
		t.Fatalf("remove evaluation: %v", err)
	}

	filename := filepath.Join(dir, "agent-remove", "job-remove.json")
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("expected evaluation file to be deleted, got err=%v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read index: %v", err)
	}
	if len(data) > 0 {
		var index map[string]string
		if err := json.Unmarshal(data, &index); err != nil {
			t.Fatalf("decode index: %v", err)
		}
		if _, ok := index["job-remove"]; ok {
			t.Fatalf("expected job to be removed from index")
		}
	}

	if _, err := store.LoadEvaluation("job-remove"); !os.IsNotExist(err) {
		t.Fatalf("expected LoadEvaluation to return not exists after deletion, got %v", err)
	}
}

func TestListRecentEvaluationsRespectsLimitAndSort(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	agentID := "agent-sorted"
	base := time.Now().Add(-5 * time.Minute)

	for i := 0; i < 3; i++ {
		snapshot := &EvaluationResults{
			JobID:     fmt.Sprintf("job-%d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		}
		if err := store.StoreEvaluation(agentID, snapshot); err != nil {
			t.Fatalf("store evaluation %d: %v", i, err)
		}
	}

	latestTwo, err := store.ListRecentEvaluations(2)
	if err != nil {
		t.Fatalf("list recent evaluations: %v", err)
	}

	if len(latestTwo) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(latestTwo))
	}

	if latestTwo[0].JobID != "job-2" || latestTwo[1].JobID != "job-1" {
		t.Fatalf("expected most recent jobs first, got %s then %s", latestTwo[0].JobID, latestTwo[1].JobID)
	}
}

func TestQueryEvaluationsFiltersByAgentWindowAndScore(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	base := time.Now()
	testCases := []struct {
		agent string
		id    string
		delta time.Duration
		score float64
	}{
		{agent: "agent-A", id: "job-1", delta: -2 * time.Minute, score: 0.6},
		{agent: "agent-A", id: "job-2", delta: -1 * time.Minute, score: 0.92},
		{agent: "agent-B", id: "job-3", delta: -30 * time.Second, score: 0.75},
	}

	for _, tc := range testCases {
		snapshot := &EvaluationResults{
			JobID:     tc.id,
			Timestamp: base.Add(tc.delta),
			Analysis: &AnalysisResult{Summary: AnalysisSummary{
				OverallScore: tc.score,
			}},
		}
		if err := store.StoreEvaluation(tc.agent, snapshot); err != nil {
			t.Fatalf("store evaluation %s: %v", tc.id, err)
		}
	}

	query := EvaluationQuery{
		AgentID:  "agent-A",
		After:    base.Add(-90 * time.Second),
		MinScore: 0.8,
	}

	evaluations, err := store.QueryEvaluations(query)
	if err != nil {
		t.Fatalf("query evaluations: %v", err)
	}

	if len(evaluations) != 1 {
		t.Fatalf("expected 1 evaluation, got %d", len(evaluations))
	}
	if evaluations[0].JobID != "job-2" {
		t.Fatalf("expected job-2 to match filters, got %s", evaluations[0].JobID)
	}

	windowQuery := EvaluationQuery{Limit: 2}
	windowResults, err := store.QueryEvaluations(windowQuery)
	if err != nil {
		t.Fatalf("query evaluations with limit: %v", err)
	}
	if len(windowResults) != 2 {
		t.Fatalf("expected 2 evaluations with limit applied, got %d", len(windowResults))
	}
	if windowResults[0].JobID != "job-3" || windowResults[1].JobID != "job-2" {
		t.Fatalf("expected jobs ordered by timestamp desc, got %s then %s", windowResults[0].JobID, windowResults[1].JobID)
	}
}

func TestQueryEvaluationsFiltersByDataset(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	snapshots := []*EvaluationResults{
		{JobID: "job-1", Timestamp: time.Now().Add(-2 * time.Minute), Config: &EvaluationConfig{DatasetPath: "./data/swe_bench/train.json", DatasetType: "swe_bench"}},
		{JobID: "job-2", Timestamp: time.Now().Add(-1 * time.Minute), Config: &EvaluationConfig{DatasetPath: "./data/custom.json", DatasetType: "custom"}},
		{JobID: "job-3", Timestamp: time.Now().Add(-30 * time.Second), Config: &EvaluationConfig{DatasetPath: "./data/swe_bench/eval.json", DatasetType: "swe_bench"}},
	}

	for _, snap := range snapshots {
		if err := store.StoreEvaluation("agent-x", snap); err != nil {
			t.Fatalf("store evaluation %s: %v", snap.JobID, err)
		}
	}

	query := EvaluationQuery{DatasetPath: "swe_bench", DatasetType: "swe_bench"}
	evaluations, err := store.QueryEvaluations(query)
	if err != nil {
		t.Fatalf("query evaluations: %v", err)
	}

	if len(evaluations) != 2 {
		t.Fatalf("expected 2 evaluations matching dataset filter, got %d", len(evaluations))
	}

	if evaluations[0].JobID != "job-3" || evaluations[1].JobID != "job-1" {
		t.Fatalf("expected swe_bench evaluations ordered by recency, got %s then %s", evaluations[0].JobID, evaluations[1].JobID)
	}
}

func TestQueryEvaluationsFiltersByAgentTags(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentDataStore(dir)

	now := time.Now()

	if _, err := store.UpsertProfile(&AgentProfile{AgentID: "agent-blue", Tags: []string{"benchmark", "Prod"}}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}
	if _, err := store.UpsertProfile(&AgentProfile{AgentID: "agent-green", Tags: []string{"staging"}}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	snapshots := []*EvaluationResults{
		{JobID: "job-blue-1", AgentID: "agent-blue", Timestamp: now.Add(-90 * time.Second)},
		{JobID: "job-blue-2", AgentID: "agent-blue", Timestamp: now.Add(-30 * time.Second)},
		{JobID: "job-green-1", AgentID: "agent-green", Timestamp: now.Add(-60 * time.Second)},
	}

	for _, snap := range snapshots {
		if err := store.StoreEvaluation(snap.AgentID, snap); err != nil {
			t.Fatalf("store evaluation %s: %v", snap.JobID, err)
		}
	}

	singleTag, err := store.QueryEvaluations(EvaluationQuery{Tags: []string{"benchmark"}})
	if err != nil {
		t.Fatalf("query evaluations by tag: %v", err)
	}

	if len(singleTag) != 2 {
		t.Fatalf("expected 2 evaluations for benchmark tag, got %d", len(singleTag))
	}
	if singleTag[0].JobID != "job-blue-2" || singleTag[1].JobID != "job-blue-1" {
		t.Fatalf("unexpected evaluation order for tag filter: %v, %v", singleTag[0].JobID, singleTag[1].JobID)
	}

	multiTag, err := store.QueryEvaluations(EvaluationQuery{Tags: []string{"benchmark", "prod"}})
	if err != nil {
		t.Fatalf("query evaluations by multi-tag: %v", err)
	}
	if len(multiTag) != 2 {
		t.Fatalf("expected both blue evaluations for multi-tag filter, got %d", len(multiTag))
	}

	none, err := store.QueryEvaluations(EvaluationQuery{Tags: []string{"nonexistent"}})
	if err != nil {
		t.Fatalf("query evaluations with missing tag: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no evaluations for missing tag filter, got %d", len(none))
	}
}
