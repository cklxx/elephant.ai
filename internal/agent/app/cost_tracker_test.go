package app

import (
	"alex/internal/agent/ports"
	"context"
	"testing"
	"time"
)

// mockCostStore is a mock implementation of CostStore for testing
type mockCostStore struct {
	records []ports.UsageRecord
}

func (m *mockCostStore) SaveUsage(ctx context.Context, record ports.UsageRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockCostStore) GetBySession(ctx context.Context, sessionID string) ([]ports.UsageRecord, error) {
	var result []ports.UsageRecord
	for _, r := range m.records {
		if r.SessionID == sessionID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockCostStore) GetByDateRange(ctx context.Context, start, end time.Time) ([]ports.UsageRecord, error) {
	var result []ports.UsageRecord
	for _, r := range m.records {
		if (r.Timestamp.After(start) || r.Timestamp.Equal(start)) &&
			(r.Timestamp.Before(end) || r.Timestamp.Equal(end)) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockCostStore) GetByModel(ctx context.Context, model string) ([]ports.UsageRecord, error) {
	var result []ports.UsageRecord
	for _, r := range m.records {
		if r.Model == model {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockCostStore) ListAll(ctx context.Context) ([]ports.UsageRecord, error) {
	return m.records, nil
}

func TestCostTracker_RecordUsage(t *testing.T) {
	store := &mockCostStore{}
	tracker := NewCostTracker(store)

	ctx := context.Background()
	usage := ports.UsageRecord{
		SessionID:    "test-session",
		Model:        "gpt-4o",
		Provider:     "openrouter",
		InputTokens:  1000,
		OutputTokens: 500,
	}

	err := tracker.RecordUsage(ctx, usage)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	if len(store.records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(store.records))
	}

	record := store.records[0]
	if record.TotalTokens != 1500 {
		t.Errorf("TotalTokens = %d, want 1500", record.TotalTokens)
	}
	if record.TotalCost == 0 {
		t.Errorf("TotalCost should be calculated, got 0")
	}
	if record.ID == "" {
		t.Errorf("ID should be generated")
	}
}

func TestCostTracker_GetSessionCost(t *testing.T) {
	store := &mockCostStore{
		records: []ports.UsageRecord{
			{
				ID:           "1",
				SessionID:    "session-1",
				Model:        "gpt-4o",
				Provider:     "openrouter",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalTokens:  1500,
				TotalCost:    0.0125,
				Timestamp:    time.Now(),
			},
			{
				ID:           "2",
				SessionID:    "session-1",
				Model:        "gpt-4o",
				Provider:     "openrouter",
				InputTokens:  2000,
				OutputTokens: 1000,
				TotalTokens:  3000,
				TotalCost:    0.025,
				Timestamp:    time.Now(),
			},
			{
				ID:           "3",
				SessionID:    "session-2",
				Model:        "gpt-4o",
				Provider:     "openrouter",
				InputTokens:  500,
				OutputTokens: 250,
				TotalTokens:  750,
				TotalCost:    0.00625,
				Timestamp:    time.Now(),
			},
		},
	}

	tracker := NewCostTracker(store)
	ctx := context.Background()

	summary, err := tracker.GetSessionCost(ctx, "session-1")
	if err != nil {
		t.Fatalf("GetSessionCost failed: %v", err)
	}

	if summary.RequestCount != 2 {
		t.Errorf("RequestCount = %d, want 2", summary.RequestCount)
	}
	// Use tolerance for floating point comparison
	tolerance := 0.000001
	if abs(summary.TotalCost-0.0375) > tolerance {
		t.Errorf("TotalCost = %f, want 0.0375", summary.TotalCost)
	}
	if summary.TotalTokens != 4500 {
		t.Errorf("TotalTokens = %d, want 4500", summary.TotalTokens)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestCostTracker_GetDailyCost(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	store := &mockCostStore{
		records: []ports.UsageRecord{
			{
				ID:           "1",
				SessionID:    "session-1",
				Model:        "gpt-4o",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalCost:    0.0125,
				Timestamp:    now,
			},
			{
				ID:           "2",
				SessionID:    "session-1",
				Model:        "gpt-4o",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalCost:    0.0125,
				Timestamp:    yesterday,
			},
		},
	}

	tracker := NewCostTracker(store)
	ctx := context.Background()

	summary, err := tracker.GetDailyCost(ctx, now)
	if err != nil {
		t.Fatalf("GetDailyCost failed: %v", err)
	}

	if summary.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (only today's records)", summary.RequestCount)
	}
	// Use tolerance for floating point comparison
	tolerance := 0.000001
	if abs(summary.TotalCost-0.0125) > tolerance {
		t.Errorf("TotalCost = %f, want 0.0125", summary.TotalCost)
	}
}

func TestCostTracker_ExportJSON(t *testing.T) {
	store := &mockCostStore{
		records: []ports.UsageRecord{
			{
				ID:           "1",
				SessionID:    "session-1",
				Model:        "gpt-4o",
				Provider:     "openrouter",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalCost:    0.0125,
				Timestamp:    time.Now(),
			},
		},
	}

	tracker := NewCostTracker(store)
	ctx := context.Background()

	data, err := tracker.Export(ctx, ports.ExportFormatJSON, ports.ExportFilter{})
	if err != nil {
		t.Fatalf("Export JSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("Export returned empty data")
	}

	// Check that it's valid JSON
	if data[0] != '[' {
		t.Errorf("JSON export should start with [, got %c", data[0])
	}
}

func TestCostTracker_ExportCSV(t *testing.T) {
	store := &mockCostStore{
		records: []ports.UsageRecord{
			{
				ID:           "1",
				SessionID:    "session-1",
				Model:        "gpt-4o",
				Provider:     "openrouter",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalCost:    0.0125,
				Timestamp:    time.Now(),
			},
		},
	}

	tracker := NewCostTracker(store)
	ctx := context.Background()

	data, err := tracker.Export(ctx, ports.ExportFormatCSV, ports.ExportFilter{})
	if err != nil {
		t.Fatalf("Export CSV failed: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("Export returned empty data")
	}

	// Check for CSV header
	csvStr := string(data)
	if !contains(csvStr, "ID") || !contains(csvStr, "SessionID") {
		t.Errorf("CSV export should contain header fields")
	}
}

func TestCostTracker_ExportWithFilter(t *testing.T) {
	store := &mockCostStore{
		records: []ports.UsageRecord{
			{
				ID:           "1",
				SessionID:    "session-1",
				Model:        "gpt-4o",
				Provider:     "openrouter",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalCost:    0.0125,
				Timestamp:    time.Now(),
			},
			{
				ID:           "2",
				SessionID:    "session-2",
				Model:        "gpt-4o-mini",
				Provider:     "openrouter",
				InputTokens:  5000,
				OutputTokens: 2500,
				TotalCost:    0.0015,
				Timestamp:    time.Now(),
			},
		},
	}

	tracker := NewCostTracker(store)
	ctx := context.Background()

	// Filter by session
	filter := ports.ExportFilter{
		SessionID: "session-1",
	}

	data, err := tracker.Export(ctx, ports.ExportFormatJSON, filter)
	if err != nil {
		t.Fatalf("Export with filter failed: %v", err)
	}

	// Should only contain session-1 record
	csvStr := string(data)
	if !contains(csvStr, "session-1") {
		t.Errorf("Export should contain session-1")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
