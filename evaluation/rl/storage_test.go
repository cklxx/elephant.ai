package rl

import (
	"testing"
	"time"
)

func TestStorage_AppendAndRead(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStorage(dir)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	traj1 := &RLTrajectory{
		ID:          "traj-1",
		QualityTier: TierGold,
		AutoScore:   90,
		ExtractedAt: now,
		Steps: []TrajectoryStep{
			{StepIndex: 0, Action: "test", Reward: 0.1},
		},
	}
	traj2 := &RLTrajectory{
		ID:          "traj-2",
		QualityTier: TierGold,
		AutoScore:   85,
		ExtractedAt: now,
		Steps: []TrajectoryStep{
			{StepIndex: 0, Action: "test2", Reward: 1.0},
		},
	}
	traj3 := &RLTrajectory{
		ID:          "traj-3",
		QualityTier: TierReject,
		AutoScore:   20,
		ExtractedAt: now,
	}

	if err := store.Append(traj1); err != nil {
		t.Fatalf("append traj1: %v", err)
	}
	if err := store.Append(traj2); err != nil {
		t.Fatalf("append traj2: %v", err)
	}
	if err := store.Append(traj3); err != nil {
		t.Fatalf("append traj3: %v", err)
	}

	// Read gold tier
	gold, err := store.ReadTier(TierGold, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("read gold: %v", err)
	}
	if len(gold) != 2 {
		t.Errorf("expected 2 gold trajectories, got %d", len(gold))
	}

	// Read reject tier
	reject, err := store.ReadTier(TierReject, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("read reject: %v", err)
	}
	if len(reject) != 1 {
		t.Errorf("expected 1 reject trajectory, got %d", len(reject))
	}

	// Read silver tier (empty)
	silver, err := store.ReadTier(TierSilver, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("read silver: %v", err)
	}
	if len(silver) != 0 {
		t.Errorf("expected 0 silver trajectories, got %d", len(silver))
	}
}

func TestStorage_DateFilter(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStorage(dir)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	day1 := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)

	if err := store.Append(&RLTrajectory{ID: "old", QualityTier: TierGold, ExtractedAt: day1}); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(&RLTrajectory{ID: "new", QualityTier: TierGold, ExtractedAt: day2}); err != nil {
		t.Fatal(err)
	}

	// Filter: only after Feb 6
	after := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)
	results, err := store.ReadTier(TierGold, after, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result after date filter, got %d", len(results))
	}
	if results[0].ID != "new" {
		t.Errorf("expected 'new', got %s", results[0].ID)
	}
}

func TestStorage_Stats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStorage(dir)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	now := time.Now()
	for i := 0; i < 3; i++ {
		if err := store.Append(&RLTrajectory{ID: "g" + string(rune('0'+i)), QualityTier: TierGold, ExtractedAt: now}); err != nil {
			t.Fatalf("append gold trajectory %d: %v", i, err)
		}
	}
	if err := store.Append(&RLTrajectory{ID: "s1", QualityTier: TierSilver, ExtractedAt: now}); err != nil {
		t.Fatalf("append silver trajectory: %v", err)
	}

	manifest, err := store.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	goldInfo := manifest.Tiers[TierGold]
	if goldInfo.TotalCount != 3 {
		t.Errorf("expected 3 gold, got %d", goldInfo.TotalCount)
	}
	silverInfo := manifest.Tiers[TierSilver]
	if silverInfo.TotalCount != 1 {
		t.Errorf("expected 1 silver, got %d", silverInfo.TotalCount)
	}
}

func TestStorage_AppendBatch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStorage(dir)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	now := time.Now()
	batch := []*RLTrajectory{
		{ID: "b1", QualityTier: TierGold, ExtractedAt: now},
		{ID: "b2", QualityTier: TierSilver, ExtractedAt: now},
		{ID: "b3", QualityTier: TierBronze, ExtractedAt: now},
	}

	if err := store.AppendBatch(batch); err != nil {
		t.Fatalf("append batch: %v", err)
	}

	manifest, err := store.Stats()
	if err != nil {
		t.Fatal(err)
	}

	total := 0
	for _, info := range manifest.Tiers {
		total += info.TotalCount
	}
	if total != 3 {
		t.Errorf("expected 3 total from batch, got %d", total)
	}
}
