package distillation

import (
	"context"
	"testing"
	"time"
)

func TestAnalyzeWeek(t *testing.T) {
	tests := []struct {
		name         string
		extractions  []DailyExtraction
		response     string
		wantPatterns int
		wantErr      bool
	}{
		{
			name: "finds patterns from extractions",
			extractions: []DailyExtraction{
				{Date: "2026-03-11", Facts: []ExtractedFact{{ID: "f1", Category: "preference"}}},
				{Date: "2026-03-12", Facts: []ExtractedFact{{ID: "f2", Category: "preference"}}},
			},
			response:     `[{"description":"prefers Go","category":"preference","evidence":["f1","f2"],"confidence":0.85}]`,
			wantPatterns: 1,
		},
		{
			name:         "no extractions returns empty",
			extractions:  []DailyExtraction{},
			response:     `[]`,
			wantPatterns: 0,
		},
		{
			name: "invalid JSON",
			extractions: []DailyExtraction{
				{Date: "2026-03-11", Facts: []ExtractedFact{{ID: "f1"}}},
			},
			response: `invalid`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockLLMClient{response: tt.response, model: "test"}
			analyzer := NewPatternAnalyzer(client, func() time.Time {
				return time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)
			})

			patterns, err := analyzer.AnalyzeWeek(context.Background(), tt.extractions)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(patterns) != tt.wantPatterns {
				t.Errorf("got %d patterns, want %d", len(patterns), tt.wantPatterns)
			}
		})
	}
}
