package memory

import (
	"context"
	"testing"
)

func TestQueryTrackerClassify(t *testing.T) {
	tracker := NewQueryTracker(t.TempDir())

	tests := []struct {
		query    string
		expected QueryCategory
	}{
		{"implement the login function", CategoryCode},
		{"fix the bug in user registration", CategoryCode},
		{"design the architecture for the new service", CategoryArchitecture},
		{"deploy to production", CategoryOps},
		{"debug the crash in session handler", CategoryDebug},
		{"document the API endpoints", CategoryDocs},
		{"what time is it", CategoryGeneral},
		{"refactor the test suite and compile", CategoryCode},
	}
	for _, tt := range tests {
		got := tracker.Classify(tt.query)
		if got != tt.expected {
			t.Errorf("Classify(%q) = %s, want %s", tt.query, got, tt.expected)
		}
	}
}

func TestQueryTrackerRecordAndLoad(t *testing.T) {
	tracker := NewQueryTracker(t.TempDir())
	ctx := context.Background()

	if err := tracker.Record(ctx, "", CategoryCode); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := tracker.Record(ctx, "", CategoryCode); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := tracker.Record(ctx, "", CategoryOps); err != nil {
		t.Fatalf("Record: %v", err)
	}

	dist, err := tracker.Load(ctx, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if dist.Total != 3 {
		t.Errorf("Total = %d, want 3", dist.Total)
	}
	if dist.Counts[CategoryCode] != 2 {
		t.Errorf("Code count = %d, want 2", dist.Counts[CategoryCode])
	}
	if dist.Counts[CategoryOps] != 1 {
		t.Errorf("Ops count = %d, want 1", dist.Counts[CategoryOps])
	}
}

func TestQueryTrackerWeights(t *testing.T) {
	tracker := NewQueryTracker(t.TempDir())
	ctx := context.Background()

	// Empty distribution should return uniform weights.
	weights, err := tracker.Weights(ctx, "")
	if err != nil {
		t.Fatalf("Weights: %v", err)
	}
	expected := 1.0 / float64(len(allCategories))
	for _, cat := range allCategories {
		if w := weights[cat]; w < expected-0.01 || w > expected+0.01 {
			t.Errorf("Empty weight[%s] = %f, want ~%f", cat, w, expected)
		}
	}

	// After recording, weights should reflect the distribution.
	_ = tracker.Record(ctx, "", CategoryCode)
	_ = tracker.Record(ctx, "", CategoryCode)
	_ = tracker.Record(ctx, "", CategoryOps)
	_ = tracker.Record(ctx, "", CategoryOps)

	weights, err = tracker.Weights(ctx, "")
	if err != nil {
		t.Fatalf("Weights: %v", err)
	}
	if w := weights[CategoryCode]; w < 0.49 || w > 0.51 {
		t.Errorf("Code weight = %f, want ~0.5", w)
	}
	if w := weights[CategoryOps]; w < 0.49 || w > 0.51 {
		t.Errorf("Ops weight = %f, want ~0.5", w)
	}
}

func TestQueryTrackerLoadMissing(t *testing.T) {
	tracker := NewQueryTracker(t.TempDir())
	dist, err := tracker.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("Load on missing: %v", err)
	}
	if dist.Total != 0 {
		t.Errorf("Total = %d, want 0", dist.Total)
	}
}

func TestParseFormatQueryStatsRoundTrip(t *testing.T) {
	dist := QueryDistribution{
		Counts: map[QueryCategory]int{
			CategoryCode:  5,
			CategoryOps:   3,
			CategoryDebug: 1,
		},
		Total: 9,
	}
	formatted := formatQueryStats(dist)
	parsed := parseQueryStats(formatted)
	if parsed.Total != 9 {
		t.Errorf("Total = %d, want 9", parsed.Total)
	}
	if parsed.Counts[CategoryCode] != 5 {
		t.Errorf("Code = %d, want 5", parsed.Counts[CategoryCode])
	}
	if parsed.Counts[CategoryOps] != 3 {
		t.Errorf("Ops = %d, want 3", parsed.Counts[CategoryOps])
	}
}
