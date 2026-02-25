package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildEvaluationQuery(t *testing.T) {
	t.Run("parses filters and tags", func(t *testing.T) {
		after := "2025-01-02T03:04:05Z"
		before := "2025-01-03T04:05:06Z"

		got, err := buildEvaluationQuery(
			"agent-1",
			15,
			after,
			before,
			0.75,
			"evaluation/custom.json",
			"file",
			"core, planner , ,ops",
		)
		if err != nil {
			t.Fatalf("buildEvaluationQuery returned error: %v", err)
		}

		expectedAfter, _ := time.Parse(time.RFC3339, after)
		expectedBefore, _ := time.Parse(time.RFC3339, before)
		wantTags := []string{"core", "planner", "ops"}

		if got.AgentID != "agent-1" {
			t.Fatalf("AgentID = %q, want %q", got.AgentID, "agent-1")
		}
		if got.Limit != 15 {
			t.Fatalf("Limit = %d, want %d", got.Limit, 15)
		}
		if !got.After.Equal(expectedAfter) {
			t.Fatalf("After = %s, want %s", got.After.Format(time.RFC3339), expectedAfter.Format(time.RFC3339))
		}
		if !got.Before.Equal(expectedBefore) {
			t.Fatalf("Before = %s, want %s", got.Before.Format(time.RFC3339), expectedBefore.Format(time.RFC3339))
		}
		if got.MinScore != 0.75 {
			t.Fatalf("MinScore = %f, want %f", got.MinScore, 0.75)
		}
		if got.DatasetPath != "evaluation/custom.json" {
			t.Fatalf("DatasetPath = %q, want %q", got.DatasetPath, "evaluation/custom.json")
		}
		if got.DatasetType != "file" {
			t.Fatalf("DatasetType = %q, want %q", got.DatasetType, "file")
		}
		if !reflect.DeepEqual(got.Tags, wantTags) {
			t.Fatalf("Tags = %#v, want %#v", got.Tags, wantTags)
		}
	})

	t.Run("invalid after timestamp", func(t *testing.T) {
		_, err := buildEvaluationQuery("agent-1", 10, "invalid-after", "", 0, "", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.HasPrefix(err.Error(), "invalid --after timestamp: ") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid before timestamp", func(t *testing.T) {
		_, err := buildEvaluationQuery("agent-1", 10, "", "invalid-before", 0, "", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.HasPrefix(err.Error(), "invalid --before timestamp: ") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSplitTags(t *testing.T) {
	t.Run("empty input returns nil", func(t *testing.T) {
		got := splitTags("")
		if got != nil {
			t.Fatalf("splitTags(\"\") = %#v, want nil", got)
		}
	})

	t.Run("trim and drop empty tags", func(t *testing.T) {
		got := splitTags("alpha, beta , ,gamma")
		want := []string{"alpha", "beta", "gamma"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("splitTags returned %#v, want %#v", got, want)
		}
	})

	t.Run("delimiters only returns empty non-nil slice", func(t *testing.T) {
		got := splitTags(" , , ")
		if got == nil {
			t.Fatal("splitTags returned nil, want empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("splitTags length = %d, want 0", len(got))
		}
	})
}
