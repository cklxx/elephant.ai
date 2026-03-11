package cost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	storage "alex/internal/domain/agent/ports/storage"
)

type failingCostStore struct {
	mockCostStore
	saveErr        error
	sessionErr     error
	dateRangeErr   error
	modelErr       error
	listAllErr     error
}

func (f *failingCostStore) SaveUsage(ctx context.Context, record storage.UsageRecord) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.mockCostStore.SaveUsage(ctx, record)
}

func (f *failingCostStore) GetBySession(ctx context.Context, sessionID string) ([]storage.UsageRecord, error) {
	if f.sessionErr != nil {
		return nil, f.sessionErr
	}
	return f.mockCostStore.GetBySession(ctx, sessionID)
}

func (f *failingCostStore) GetByDateRange(ctx context.Context, start, end time.Time) ([]storage.UsageRecord, error) {
	if f.dateRangeErr != nil {
		return nil, f.dateRangeErr
	}
	return f.mockCostStore.GetByDateRange(ctx, start, end)
}

func (f *failingCostStore) GetByModel(ctx context.Context, model string) ([]storage.UsageRecord, error) {
	if f.modelErr != nil {
		return nil, f.modelErr
	}
	return f.mockCostStore.GetByModel(ctx, model)
}

func (f *failingCostStore) ListAll(ctx context.Context) ([]storage.UsageRecord, error) {
	if f.listAllErr != nil {
		return nil, f.listAllErr
	}
	return f.mockCostStore.ListAll(ctx)
}

func TestCostTrackerGetSessionStats(t *testing.T) {
	t.Run("empty session", func(t *testing.T) {
		tracker := NewCostTracker(&mockCostStore{})
		stats, err := tracker.GetSessionStats(context.Background(), "missing")
		if err != nil {
			t.Fatalf("GetSessionStats() error = %v", err)
		}
		if stats.SessionID != "missing" || stats.RequestCount != 0 {
			t.Fatalf("stats = %#v, want empty stats for session", stats)
		}
		if stats.ByModel == nil || stats.ByProvider == nil {
			t.Fatalf("stats maps should be initialized: %#v", stats)
		}
	})

	t.Run("aggregates duration and buckets", func(t *testing.T) {
		start := time.Date(2026, time.March, 11, 9, 0, 0, 0, time.UTC)
		store := &mockCostStore{
			records: []storage.UsageRecord{
				{SessionID: "s1", Model: "gpt-5", Provider: "openai", TotalTokens: 100, TotalCost: 1.25, Timestamp: start.Add(2 * time.Hour)},
				{SessionID: "s1", Model: "gpt-5", Provider: "openai", TotalTokens: 50, TotalCost: 0.75, Timestamp: start},
				{SessionID: "s1", Model: "claude", Provider: "anthropic", TotalTokens: 30, TotalCost: 0.50, Timestamp: start.Add(30 * time.Minute)},
			},
		}
		tracker := NewCostTracker(store)
		stats, err := tracker.GetSessionStats(context.Background(), "s1")
		if err != nil {
			t.Fatalf("GetSessionStats() error = %v", err)
		}
		if stats.RequestCount != 3 || stats.TotalTokens != 180 {
			t.Fatalf("stats = %#v, want aggregated counts", stats)
		}
		if stats.Duration != 2*time.Hour {
			t.Fatalf("Duration = %v, want 2h", stats.Duration)
		}
		if stats.ByModel["gpt-5"] != 2.0 || stats.ByProvider["anthropic"] != 0.50 {
			t.Fatalf("bucketed stats = %#v %#v, want aggregated model/provider totals", stats.ByModel, stats.ByProvider)
		}
	})
}

func TestCostTrackerRangeAndExportBranches(t *testing.T) {
	now := time.Date(2026, time.March, 11, 8, 0, 0, 0, time.UTC)
	store := &mockCostStore{
		records: []storage.UsageRecord{
			{SessionID: "s1", Model: "gpt-5", Provider: "openai", TotalTokens: 100, TotalCost: 1.0, Timestamp: now},
			{SessionID: "s2", Model: "claude", Provider: "anthropic", TotalTokens: 50, TotalCost: 2.0, Timestamp: now.AddDate(0, 1, 0)},
		},
	}
	tracker := NewCostTracker(store)

	monthly, err := tracker.GetMonthlyCost(context.Background(), 2026, int(time.March))
	if err != nil {
		t.Fatalf("GetMonthlyCost() error = %v", err)
	}
	if monthly.RequestCount != 1 || monthly.TotalCost != 1.0 {
		t.Fatalf("monthly = %#v, want only March records", monthly)
	}

	rangeSummary, err := tracker.GetDateRangeCost(context.Background(), now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetDateRangeCost() error = %v", err)
	}
	if rangeSummary.RequestCount != 1 || rangeSummary.TotalCost != 1.0 {
		t.Fatalf("rangeSummary = %#v, want bounded range results", rangeSummary)
	}

	data, err := tracker.Export(context.Background(), storage.ExportFormatJSON, storage.ExportFilter{
		StartDate: now.Add(-time.Minute),
		EndDate:   now.Add(2 * time.Minute),
		Provider:  "openai",
		Model:     "gpt-5",
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	var records []storage.UsageRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(records) != 1 || records[0].SessionID != "s1" {
		t.Fatalf("Export() records = %#v, want only filtered s1 record", records)
	}

	if _, err := tracker.Export(context.Background(), storage.ExportFormat("xml"), storage.ExportFilter{}); err == nil {
		t.Fatal("Export() error = nil, want unsupported format error")
	}
}

func TestCostTrackerErrorWrapping(t *testing.T) {
	saveErr := errors.New("save failed")
	tracker := NewCostTracker(&failingCostStore{saveErr: saveErr})
	if err := tracker.RecordUsage(context.Background(), storage.UsageRecord{SessionID: "s", Model: "gpt-5"}); err == nil || !strings.Contains(err.Error(), "save usage") {
		t.Fatalf("RecordUsage() error = %v, want wrapped save error", err)
	}

	sessionErr := errors.New("session lookup failed")
	tracker = NewCostTracker(&failingCostStore{sessionErr: sessionErr})
	if _, err := tracker.GetSessionCost(context.Background(), "s"); err == nil || !strings.Contains(err.Error(), "get session records") {
		t.Fatalf("GetSessionCost() error = %v, want wrapped session error", err)
	}
}
