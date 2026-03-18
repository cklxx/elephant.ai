package distillation

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestStoreDailyExtraction(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ctx := context.Background()

	ext := &DailyExtraction{
		Date:   "2026-03-18",
		Facts:  []ExtractedFact{{ID: "f1", Content: "test fact", Category: "decision", Confidence: 0.9}},
		Tokens: 100,
	}

	tests := []struct {
		name    string
		op      func() error
		check   func(t *testing.T)
	}{
		{
			name: "save and load",
			op:   func() error { return store.SaveDailyExtraction(ctx, ext) },
			check: func(t *testing.T) {
				loaded, err := store.LoadDailyExtraction(ctx, "2026-03-18")
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				if len(loaded.Facts) != 1 || loaded.Facts[0].ID != "f1" {
					t.Errorf("unexpected loaded extraction: %+v", loaded)
				}
			},
		},
		{
			name: "load nonexistent returns error",
			op:   func() error { return nil },
			check: func(t *testing.T) {
				_, err := store.LoadDailyExtraction(ctx, "1999-01-01")
				if err == nil {
					t.Error("expected error for nonexistent date")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.op(); err != nil {
				t.Fatalf("op: %v", err)
			}
			tt.check(t)
		})
	}
}

func TestStoreListDailyExtractions(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ctx := context.Background()

	dates := []string{"2026-03-15", "2026-03-16", "2026-03-17", "2026-03-18"}
	for _, d := range dates {
		err := store.SaveDailyExtraction(ctx, &DailyExtraction{Date: d, Facts: []ExtractedFact{{ID: d}}})
		if err != nil {
			t.Fatalf("save %s: %v", d, err)
		}
	}

	from := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 17, 23, 59, 59, 0, time.UTC)
	results, err := store.ListDailyExtractions(ctx, from, to)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestStoreWeeklyPatterns(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ctx := context.Background()

	patterns := []WeeklyPattern{{ID: "wp1", Description: "test", Confidence: 0.9}}
	if err := store.SaveWeeklyPatterns(ctx, "2026-03-11", patterns); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.LoadWeeklyPatterns(ctx, "2026-03-11")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "wp1" {
		t.Errorf("unexpected patterns: %+v", loaded)
	}
}

func TestStorePersonalityModel(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ctx := context.Background()

	model := &PersonalityModel{
		UserID:      "user1",
		Preferences: map[string]string{"lang": "Go"},
		LastUpdated: time.Now(),
	}
	if err := store.SavePersonalityModel(ctx, model); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.LoadPersonalityModel(ctx, "user1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Preferences["lang"] != "Go" {
		t.Errorf("unexpected preference: %v", loaded.Preferences)
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			date := time.Date(2026, 3, 1+idx, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			ext := &DailyExtraction{Date: date, Facts: []ExtractedFact{{ID: date}}}
			_ = store.SaveDailyExtraction(ctx, ext)
			_, _ = store.LoadDailyExtraction(ctx, date)
		}(i)
	}
	wg.Wait()
}
