package ports

import (
	"context"
	"time"
)

// CostTracker tracks token usage and costs across LLM interactions
type CostTracker interface {
	// RecordUsage records a single LLM API call usage
	RecordUsage(ctx context.Context, usage UsageRecord) error

	// GetSessionCost returns total cost for a specific session
	GetSessionCost(ctx context.Context, sessionID string) (*CostSummary, error)

	// GetSessionStats returns detailed statistics for a specific session
	// This includes total tokens, costs, and request counts
	GetSessionStats(ctx context.Context, sessionID string) (*SessionStats, error)

	// GetDailyCost returns aggregated cost for a specific day
	GetDailyCost(ctx context.Context, date time.Time) (*CostSummary, error)

	// GetMonthlyCost returns aggregated cost for a specific month
	GetMonthlyCost(ctx context.Context, year int, month int) (*CostSummary, error)

	// GetDateRangeCost returns cost for a date range
	GetDateRangeCost(ctx context.Context, start, end time.Time) (*CostSummary, error)

	// Export exports usage records in specified format
	Export(ctx context.Context, format ExportFormat, filter ExportFilter) ([]byte, error)
}

// CostStore persists cost and usage data
type CostStore interface {
	// SaveUsage saves a usage record
	SaveUsage(ctx context.Context, record UsageRecord) error

	// GetBySession retrieves all usage records for a session
	GetBySession(ctx context.Context, sessionID string) ([]UsageRecord, error)

	// GetByDateRange retrieves records within a date range
	GetByDateRange(ctx context.Context, start, end time.Time) ([]UsageRecord, error)

	// GetByModel retrieves records for a specific model
	GetByModel(ctx context.Context, model string) ([]UsageRecord, error)

	// ListAll retrieves all usage records
	ListAll(ctx context.Context) ([]UsageRecord, error)
}

// UsageRecord represents a single LLM usage event
type UsageRecord struct {
	ID              string         `json:"id"`
	SessionID       string         `json:"session_id"`
	Model           string         `json:"model"`
	Provider        string         `json:"provider"`
	InputTokens     int            `json:"input_tokens"`
	OutputTokens    int            `json:"output_tokens"`
	TotalTokens     int            `json:"total_tokens"`
	InputCost       float64        `json:"input_cost"`
	OutputCost      float64        `json:"output_cost"`
	TotalCost       float64        `json:"total_cost"`
	Timestamp       time.Time      `json:"timestamp"`
	RequestMetadata map[string]any `json:"request_metadata,omitempty"`
}

// CostSummary aggregates cost and usage data
type CostSummary struct {
	TotalCost    float64            `json:"total_cost"`
	InputTokens  int                `json:"input_tokens"`
	OutputTokens int                `json:"output_tokens"`
	TotalTokens  int                `json:"total_tokens"`
	RequestCount int                `json:"request_count"`
	ByModel      map[string]float64 `json:"by_model"`
	ByProvider   map[string]float64 `json:"by_provider"`
	StartTime    time.Time          `json:"start_time"`
	EndTime      time.Time          `json:"end_time"`
}

// SessionStats provides detailed statistics for a session
type SessionStats struct {
	SessionID    string             `json:"session_id"`
	TotalCost    float64            `json:"total_cost"`
	InputTokens  int                `json:"input_tokens"`
	OutputTokens int                `json:"output_tokens"`
	TotalTokens  int                `json:"total_tokens"`
	RequestCount int                `json:"request_count"`
	ByModel      map[string]float64 `json:"by_model,omitempty"`
	ByProvider   map[string]float64 `json:"by_provider,omitempty"`
	FirstRequest time.Time          `json:"first_request"`
	LastRequest  time.Time          `json:"last_request"`
	Duration     time.Duration      `json:"duration"`
}

// ExportFormat defines export format types
type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"
)

// ExportFilter defines filters for export
type ExportFilter struct {
	SessionID string
	Model     string
	Provider  string
	StartDate time.Time
	EndDate   time.Time
}

// ModelPricing holds pricing information per 1K tokens
type ModelPricing struct {
	InputPer1K  float64
	OutputPer1K float64
}

// GetModelPricing returns pricing for a given model
func GetModelPricing(model string) ModelPricing {
	// Pricing as of 2025
	pricingMap := map[string]ModelPricing{
		"gpt-4":                             {InputPer1K: 0.03, OutputPer1K: 0.06},
		"gpt-4-turbo":                       {InputPer1K: 0.01, OutputPer1K: 0.03},
		"gpt-4o":                            {InputPer1K: 0.005, OutputPer1K: 0.015},
		"gpt-4o-mini":                       {InputPer1K: 0.00015, OutputPer1K: 0.0006},
		"deepseek-chat":                     {InputPer1K: 0.00014, OutputPer1K: 0.00028},
		"deepseek-reasoner":                 {InputPer1K: 0.00055, OutputPer1K: 0.00219},
		"anthropic/claude-3-5-sonnet":       {InputPer1K: 0.003, OutputPer1K: 0.015},
		"anthropic/claude-3-opus":           {InputPer1K: 0.015, OutputPer1K: 0.075},
		"meta-llama/llama-3.1-70b-instruct": {InputPer1K: 0.0005, OutputPer1K: 0.0008},
	}

	if pricing, ok := pricingMap[model]; ok {
		return pricing
	}

	// Default pricing for unknown models
	return ModelPricing{InputPer1K: 0.001, OutputPer1K: 0.002}
}

// CalculateCost calculates cost based on token usage and model
func CalculateCost(inputTokens, outputTokens int, model string) (inputCost, outputCost, totalCost float64) {
	pricing := GetModelPricing(model)

	inputCost = float64(inputTokens) / 1000.0 * pricing.InputPer1K
	outputCost = float64(outputTokens) / 1000.0 * pricing.OutputPer1K
	totalCost = inputCost + outputCost

	return
}
