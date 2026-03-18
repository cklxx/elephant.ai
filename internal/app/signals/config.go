package signals

import "time"

// Config holds signal graph configuration.
type Config struct {
	Enabled            bool          `yaml:"enabled"`
	BufferSize         int           `yaml:"buffer_size"`
	LLMBudgetPerHour   int           `yaml:"llm_budget_per_hour"`
	SummarizeThreshold int           `yaml:"summarize_threshold"`
	QueueThreshold     int           `yaml:"queue_threshold"`
	NotifyNowThreshold int           `yaml:"notify_now_threshold"`
	EscalateThreshold  int           `yaml:"escalate_threshold"`
	BudgetWindow       time.Duration `yaml:"budget_window"`
	BudgetMax          int           `yaml:"budget_max"`
	QuietHoursStart    int           `yaml:"quiet_hours_start"`
	QuietHoursEnd      int           `yaml:"quiet_hours_end"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:            true,
		BufferSize:         500,
		LLMBudgetPerHour:   50,
		SummarizeThreshold: 40,
		QueueThreshold:     60,
		NotifyNowThreshold: 80,
		EscalateThreshold:  90,
		BudgetWindow:       10 * time.Minute,
		BudgetMax:          20,
		QuietHoursStart:    0,
		QuietHoursEnd:      0,
	}
}
