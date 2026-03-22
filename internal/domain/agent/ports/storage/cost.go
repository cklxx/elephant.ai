package storage

import (
	"context"
	"time"
)

type CostTracker interface {
	RecordUsage(ctx context.Context, usage UsageRecord) error
	GetSessionCost(ctx context.Context, sessionID string) (*CostSummary, error)
	GetSessionStats(ctx context.Context, sessionID string) (*SessionStats, error)
	GetDailyCost(ctx context.Context, date time.Time) (*CostSummary, error)
	GetMonthlyCost(ctx context.Context, year int, month int) (*CostSummary, error)
	GetDateRangeCost(ctx context.Context, start, end time.Time) (*CostSummary, error)
	Export(ctx context.Context, format ExportFormat, filter ExportFilter) ([]byte, error)
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
