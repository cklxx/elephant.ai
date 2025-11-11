package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParseRetentionDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"2w", 14 * 24 * time.Hour},
		{"48h", 48 * time.Hour},
		{"90m", 90 * time.Minute},
		{"15", 15 * 24 * time.Hour},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := parseRetentionDuration(tc.input)
			if err != nil {
				t.Fatalf("parseRetentionDuration(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseRetentionDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSelectSessionsForCleanup(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 11, 11, 12, 0, 0, 0, time.UTC)
	metas := []sessionMetadata{
		{ID: "recent", UpdatedAt: now.Add(-6 * time.Hour)},
		{ID: "yesterday", UpdatedAt: now.Add(-24 * time.Hour)},
		{ID: "lastWeek", UpdatedAt: now.Add(-7 * 24 * time.Hour)},
		{ID: "lastMonth", UpdatedAt: now.Add(-33 * 24 * time.Hour)},
		{ID: "twoMonths", UpdatedAt: now.Add(-65 * 24 * time.Hour)},
	}

	opts := sessionCleanupOptions{
		olderThan:  30 * 24 * time.Hour,
		keepLatest: 2,
	}
	targets := selectSessionsForCleanup(metas, opts, now)

	gotIDs := make([]string, len(targets))
	for i, target := range targets {
		gotIDs[i] = target.ID
	}
	wantIDs := []string{"lastMonth", "twoMonths"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("selectSessionsForCleanup mismatch, got %v want %v", gotIDs, wantIDs)
	}

	optsNoAge := sessionCleanupOptions{olderThan: 0, keepLatest: 1}
	noAgeTargets := selectSessionsForCleanup(metas, optsNoAge, now)
	if len(noAgeTargets) != len(metas)-1 {
		t.Fatalf("expected %d entries, got %d", len(metas)-1, len(noAgeTargets))
	}
}
