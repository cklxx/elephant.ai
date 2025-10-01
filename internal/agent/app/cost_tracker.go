package app

import (
	"alex/internal/agent/ports"
	"alex/internal/utils"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// costTracker implements the CostTracker interface
type costTracker struct {
	store  ports.CostStore
	logger *utils.Logger
}

// NewCostTracker creates a new cost tracker instance
func NewCostTracker(store ports.CostStore) ports.CostTracker {
	return &costTracker{
		store:  store,
		logger: utils.NewComponentLogger("CostTracker"),
	}
}

// RecordUsage records a single LLM API call usage
func (t *costTracker) RecordUsage(ctx context.Context, usage ports.UsageRecord) error {
	// Generate ID if not provided
	if usage.ID == "" {
		usage.ID = fmt.Sprintf("usage-%d-%s", time.Now().UnixNano(), usage.SessionID)
	}

	// Set timestamp if not provided
	if usage.Timestamp.IsZero() {
		usage.Timestamp = time.Now()
	}

	// Calculate total tokens if not set
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	// Calculate costs if not already set
	if usage.TotalCost == 0 {
		inputCost, outputCost, totalCost := ports.CalculateCost(
			usage.InputTokens,
			usage.OutputTokens,
			usage.Model,
		)
		usage.InputCost = inputCost
		usage.OutputCost = outputCost
		usage.TotalCost = totalCost
	}

	t.logger.Debug("Recording usage: session=%s, model=%s, tokens=%d/%d, cost=$%.6f",
		usage.SessionID, usage.Model, usage.InputTokens, usage.OutputTokens, usage.TotalCost)

	if err := t.store.SaveUsage(ctx, usage); err != nil {
		t.logger.Error("Failed to save usage: %v", err)
		return fmt.Errorf("save usage: %w", err)
	}

	return nil
}

// GetSessionCost returns total cost for a specific session
func (t *costTracker) GetSessionCost(ctx context.Context, sessionID string) (*ports.CostSummary, error) {
	records, err := t.store.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session records: %w", err)
	}

	return t.aggregateRecords(records), nil
}

// GetDailyCost returns aggregated cost for a specific day
func (t *costTracker) GetDailyCost(ctx context.Context, date time.Time) (*ports.CostSummary, error) {
	// Normalize to start and end of day
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)

	records, err := t.store.GetByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily records: %w", err)
	}

	summary := t.aggregateRecords(records)
	summary.StartTime = start
	summary.EndTime = end

	return summary, nil
}

// GetMonthlyCost returns aggregated cost for a specific month
func (t *costTracker) GetMonthlyCost(ctx context.Context, year int, month int) (*ports.CostSummary, error) {
	// First day of month
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	// First day of next month
	end := start.AddDate(0, 1, 0)

	records, err := t.store.GetByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("get monthly records: %w", err)
	}

	summary := t.aggregateRecords(records)
	summary.StartTime = start
	summary.EndTime = end

	return summary, nil
}

// GetDateRangeCost returns cost for a date range
func (t *costTracker) GetDateRangeCost(ctx context.Context, start, end time.Time) (*ports.CostSummary, error) {
	records, err := t.store.GetByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("get date range records: %w", err)
	}

	summary := t.aggregateRecords(records)
	summary.StartTime = start
	summary.EndTime = end

	return summary, nil
}

// Export exports usage records in specified format
func (t *costTracker) Export(ctx context.Context, format ports.ExportFormat, filter ports.ExportFilter) ([]byte, error) {
	// Get filtered records
	records, err := t.getFilteredRecords(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("get filtered records: %w", err)
	}

	switch format {
	case ports.ExportFormatJSON:
		return t.exportJSON(records)
	case ports.ExportFormatCSV:
		return t.exportCSV(records)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// aggregateRecords aggregates usage records into a summary
func (t *costTracker) aggregateRecords(records []ports.UsageRecord) *ports.CostSummary {
	summary := &ports.CostSummary{
		ByModel:    make(map[string]float64),
		ByProvider: make(map[string]float64),
	}

	if len(records) == 0 {
		return summary
	}

	// Find time range
	summary.StartTime = records[0].Timestamp
	summary.EndTime = records[0].Timestamp

	for _, record := range records {
		summary.TotalCost += record.TotalCost
		summary.InputTokens += record.InputTokens
		summary.OutputTokens += record.OutputTokens
		summary.TotalTokens += record.TotalTokens
		summary.RequestCount++

		// Aggregate by model
		summary.ByModel[record.Model] += record.TotalCost

		// Aggregate by provider
		summary.ByProvider[record.Provider] += record.TotalCost

		// Update time range
		if record.Timestamp.Before(summary.StartTime) {
			summary.StartTime = record.Timestamp
		}
		if record.Timestamp.After(summary.EndTime) {
			summary.EndTime = record.Timestamp
		}
	}

	return summary
}

// getFilteredRecords retrieves records based on filter criteria
func (t *costTracker) getFilteredRecords(ctx context.Context, filter ports.ExportFilter) ([]ports.UsageRecord, error) {
	var records []ports.UsageRecord
	var err error

	// Priority: SessionID > DateRange > Model > All
	if filter.SessionID != "" {
		records, err = t.store.GetBySession(ctx, filter.SessionID)
	} else if !filter.StartDate.IsZero() || !filter.EndDate.IsZero() {
		start := filter.StartDate
		end := filter.EndDate
		if start.IsZero() {
			start = time.Unix(0, 0) // Beginning of time
		}
		if end.IsZero() {
			end = time.Now() // Now
		}
		records, err = t.store.GetByDateRange(ctx, start, end)
	} else if filter.Model != "" {
		records, err = t.store.GetByModel(ctx, filter.Model)
	} else {
		records, err = t.store.ListAll(ctx)
	}

	if err != nil {
		return nil, err
	}

	// Apply additional filters
	filtered := make([]ports.UsageRecord, 0, len(records))
	for _, record := range records {
		if filter.Provider != "" && record.Provider != filter.Provider {
			continue
		}
		if filter.Model != "" && record.Model != filter.Model {
			continue
		}
		filtered = append(filtered, record)
	}

	return filtered, nil
}

// exportJSON exports records as JSON
func (t *costTracker) exportJSON(records []ports.UsageRecord) ([]byte, error) {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	return data, nil
}

// exportCSV exports records as CSV
func (t *costTracker) exportCSV(records []ports.UsageRecord) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"ID", "Timestamp", "SessionID", "Provider", "Model",
		"InputTokens", "OutputTokens", "TotalTokens",
		"InputCost", "OutputCost", "TotalCost",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header: %w", err)
	}

	// Write records
	for _, record := range records {
		row := []string{
			record.ID,
			record.Timestamp.Format(time.RFC3339),
			record.SessionID,
			record.Provider,
			record.Model,
			fmt.Sprintf("%d", record.InputTokens),
			fmt.Sprintf("%d", record.OutputTokens),
			fmt.Sprintf("%d", record.TotalTokens),
			fmt.Sprintf("%.6f", record.InputCost),
			fmt.Sprintf("%.6f", record.OutputCost),
			fmt.Sprintf("%.6f", record.TotalCost),
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV: %w", err)
	}

	return []byte(buf.String()), nil
}
