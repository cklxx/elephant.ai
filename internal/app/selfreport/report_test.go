package selfreport

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type staticStatsProvider struct {
	stats *Stats
	err   error
}

func (s *staticStatsProvider) GetStats(_ context.Context) (*Stats, error) {
	return s.stats, s.err
}

func TestSelfReportSpec(t *testing.T) {
	tests := []struct {
		name         string
		stats        *Stats
		statsErr     error
		wantErr      bool
		wantEmpty    bool
		wantContains []string
	}{
		{
			name: "full report",
			stats: &Stats{
				TasksCompleted:  5,
				TasksInProgress: 2,
				AutoActed:       10,
				Escalated:       1,
				Corrected:       0,
				LLMCostUSD:      1.23,
				SignalVolume:    42,
				Lessons:         []string{"Retry on 429"},
			},
			wantContains: []string{"Activity", "Decision Engine", "Cost", "$1.23", "Retry on 429"},
		},
		{
			name:      "quiet week",
			stats:     &Stats{},
			wantEmpty: true,
		},
		{
			name:     "provider error",
			statsErr: errors.New("db down"),
			wantErr:  true,
		},
		{
			name: "no lessons",
			stats: &Stats{
				TasksCompleted: 1,
				LLMCostUSD:     0.50,
			},
			wantContains: []string{"Activity", "What I Learned", "_none_"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := NewSelfReportSpec(&staticStatsProvider{stats: tt.stats, err: tt.statsErr})
			if spec.Name() != "Self-Report" {
				t.Errorf("Name() = %q", spec.Name())
			}

			content, err := spec.Generate(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Generate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			formatted := spec.Format(content)
			if tt.wantEmpty {
				if formatted != "Quiet week — no tasks processed" {
					t.Errorf("empty state = %q", formatted)
				}
				return
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(formatted, s) {
					t.Errorf("formatted missing %q in:\n%s", s, formatted)
				}
			}
		})
	}
}
