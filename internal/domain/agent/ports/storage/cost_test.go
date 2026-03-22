package storage

import (
	"testing"
	"time"
)

func TestUsageRecord(t *testing.T) {
	record := UsageRecord{
		ID:           "test-1",
		SessionID:    "session-123",
		Model:        "gpt-4o",
		Provider:     "openrouter",
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
		InputCost:    0.005,
		OutputCost:   0.0075,
		TotalCost:    0.0125,
		Timestamp:    time.Now(),
	}

	if record.ID != "test-1" {
		t.Errorf("ID = %s, want test-1", record.ID)
	}
	if record.TotalTokens != record.InputTokens+record.OutputTokens {
		t.Errorf("TotalTokens mismatch: %d != %d + %d", record.TotalTokens, record.InputTokens, record.OutputTokens)
	}
}

func TestCostSummary(t *testing.T) {
	summary := CostSummary{
		TotalCost:    0.0125,
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
		RequestCount: 1,
		ByModel: map[string]float64{
			"gpt-4o": 0.0125,
		},
		ByProvider: map[string]float64{
			"openrouter": 0.0125,
		},
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	if summary.TotalCost != 0.0125 {
		t.Errorf("TotalCost = %f, want 0.0125", summary.TotalCost)
	}
	if summary.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", summary.RequestCount)
	}
	if len(summary.ByModel) != 1 {
		t.Errorf("len(ByModel) = %d, want 1", len(summary.ByModel))
	}
}
