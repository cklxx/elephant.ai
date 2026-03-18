package signals

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Enabled", cfg.Enabled, true},
		{"BufferSize", cfg.BufferSize, 500},
		{"LLMBudgetPerHour", cfg.LLMBudgetPerHour, 50},
		{"SummarizeThreshold", cfg.SummarizeThreshold, 40},
		{"QueueThreshold", cfg.QueueThreshold, 60},
		{"NotifyNowThreshold", cfg.NotifyNowThreshold, 80},
		{"EscalateThreshold", cfg.EscalateThreshold, 90},
		{"BudgetWindow", cfg.BudgetWindow, 10 * time.Minute},
		{"BudgetMax", cfg.BudgetMax, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}
