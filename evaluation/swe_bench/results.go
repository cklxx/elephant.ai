package swe_bench

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ResultWriterImpl implements the ResultWriter interface
type ResultWriterImpl struct {
	// No state needed for this implementation
}

// NewResultWriter creates a new result writer
func NewResultWriter() *ResultWriterImpl {
	return &ResultWriterImpl{}
}

// WriteResults writes batch results to storage
func (rw *ResultWriterImpl) WriteResults(ctx context.Context, result *BatchResult, path string) error {
	cleanedPath, err := sanitizeOutputPath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cleanedPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write full batch results
	batchFile := filepath.Join(cleanedPath, "batch_results.json")
	if err := rw.writeJSONFile(batchFile, result); err != nil {
		return fmt.Errorf("failed to write batch results: %w", err)
	}

	// Write predictions in SWE-bench format
	predsFile := filepath.Join(cleanedPath, "preds.json")
	if err := rw.writePredictions(predsFile, result.Results); err != nil {
		return fmt.Errorf("failed to write predictions: %w", err)
	}

	// Write summary
	summaryFile := filepath.Join(cleanedPath, "summary.json")
	if err := rw.writeSummary(summaryFile, result); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	// Write detailed results
	detailedFile := filepath.Join(cleanedPath, "detailed_results.json")
	if err := rw.writeDetailedResults(detailedFile, result.Results); err != nil {
		return fmt.Errorf("failed to write detailed results: %w", err)
	}

	return nil
}

// WritePartialResults writes partial results during processing
func (rw *ResultWriterImpl) WritePartialResults(ctx context.Context, results []WorkerResult, path string) error {
	cleanedPath, err := sanitizeOutputPath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cleanedPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write partial predictions
	predsFile := filepath.Join(cleanedPath, "preds_partial.json")
	if err := rw.writePredictions(predsFile, results); err != nil {
		return fmt.Errorf("failed to write partial predictions: %w", err)
	}

	// Write partial detailed results
	detailedFile := filepath.Join(cleanedPath, "detailed_results_partial.json")
	if err := rw.writeDetailedResults(detailedFile, results); err != nil {
		return fmt.Errorf("failed to write partial detailed results: %w", err)
	}

	return nil
}

// ReadResults reads previously saved results
func (rw *ResultWriterImpl) ReadResults(ctx context.Context, path string) (*BatchResult, error) {
	cleanedPath, err := sanitizeOutputPath(path)
	if err != nil {
		return nil, err
	}

	batchFile := filepath.Join(cleanedPath, "batch_results.json")

	data, err := os.ReadFile(batchFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch results file: %w", err)
	}

	var result BatchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch results: %w", err)
	}

	return &result, nil
}

// AppendResult appends a single result to the output
func (rw *ResultWriterImpl) AppendResult(ctx context.Context, result WorkerResult, path string) error {
	cleanedPath, err := sanitizeOutputPath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cleanedPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Append to streaming results file
	streamFile := filepath.Join(cleanedPath, "streaming_results.jsonl")
	file, err := os.OpenFile(streamFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open streaming results file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close streaming results file: %v", err)
		}
	}()

	// Write result as JSON line
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	if _, err := file.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// writePredictions writes predictions in SWE-bench format
func (rw *ResultWriterImpl) writePredictions(filePath string, results []WorkerResult) error {
	predictions := make([]SWEBenchPrediction, 0, len(results))

	for _, result := range results {
		prediction := SWEBenchPrediction{
			InstanceID:   result.InstanceID,
			Solution:     result.Solution,
			Explanation:  result.Explanation,
			FilesChanged: result.FilesChanged,
			Commands:     result.Commands,

			// Metadata
			Status:     string(result.Status),
			Duration:   result.Duration.Seconds(),
			TokensUsed: result.TokensUsed,
			Cost:       result.Cost,
			Error:      result.Error,
			ErrorType:  result.ErrorType,
			RetryCount: result.RetryCount,
		}

		predictions = append(predictions, prediction)
	}

	return rw.writeJSONFile(filePath, predictions)
}

// writeSummary writes a summary of the batch results
func (rw *ResultWriterImpl) writeSummary(filePath string, result *BatchResult) error {
	summary := BatchSummary{
		Timestamp:      result.EndTime.Format(time.RFC3339),
		Duration:       result.Duration.String(),
		TotalTasks:     result.TotalTasks,
		CompletedTasks: result.CompletedTasks,
		FailedTasks:    result.FailedTasks,
		SuccessRate:    result.SuccessRate,
		TotalTokens:    result.TotalTokens,
		TotalCost:      result.TotalCost,
		AvgDuration:    result.AvgDuration.String(),
		ErrorSummary:   result.ErrorSummary,

		// Configuration summary
		ModelName:     result.Config.Agent.Model.Name,
		NumWorkers:    result.Config.NumWorkers,
		DatasetType:   result.Config.Instances.Type,
		DatasetSubset: result.Config.Instances.Subset,
		DatasetSplit:  result.Config.Instances.Split,
	}

	return rw.writeJSONFile(filePath, summary)
}

// writeDetailedResults writes detailed results with full trace information
func (rw *ResultWriterImpl) writeDetailedResults(filePath string, results []WorkerResult) error {
	return rw.writeJSONFile(filePath, results)
}

// writeJSONFile writes data to a JSON file
func (rw *ResultWriterImpl) writeJSONFile(filePath string, data interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close file %s: %v", filePath, err)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// SWEBenchPrediction represents a prediction in SWE-bench format
type SWEBenchPrediction struct {
	InstanceID   string   `json:"instance_id"`
	Solution     string   `json:"solution"`
	Explanation  string   `json:"explanation,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
	Commands     []string `json:"commands,omitempty"`

	// Metadata (not part of standard SWE-bench format)
	Status     string  `json:"status,omitempty"`
	Duration   float64 `json:"duration_seconds,omitempty"`
	TokensUsed int     `json:"tokens_used,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
	Error      string  `json:"error,omitempty"`
	ErrorType  string  `json:"error_type,omitempty"`
	RetryCount int     `json:"retry_count,omitempty"`
}

// BatchSummary represents a summary of batch processing results
type BatchSummary struct {
	Timestamp      string         `json:"timestamp"`
	Duration       string         `json:"duration"`
	TotalTasks     int            `json:"total_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	FailedTasks    int            `json:"failed_tasks"`
	SuccessRate    float64        `json:"success_rate"`
	TotalTokens    int            `json:"total_tokens"`
	TotalCost      float64        `json:"total_cost"`
	AvgDuration    string         `json:"avg_duration"`
	ErrorSummary   map[string]int `json:"error_summary"`

	// Configuration summary
	ModelName     string `json:"model_name"`
	NumWorkers    int    `json:"num_workers"`
	DatasetType   string `json:"dataset_type"`
	DatasetSubset string `json:"dataset_subset"`
	DatasetSplit  string `json:"dataset_split"`
}

// ExportToCSV exports results to CSV format
func (rw *ResultWriterImpl) ExportToCSV(filePath string, results []WorkerResult) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close CSV file: %v", err)
		}
	}()

	// Write CSV header
	header := "instance_id,status,duration_seconds,tokens_used,cost,error_type,retry_count\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, result := range results {
		row := fmt.Sprintf("%s,%s,%.2f,%d,%.4f,%s,%d\n",
			result.InstanceID,
			result.Status,
			result.Duration.Seconds(),
			result.TokensUsed,
			result.Cost,
			result.ErrorType,
			result.RetryCount,
		)
		if _, err := file.WriteString(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// ExportToMarkdown exports results to Markdown format
func (rw *ResultWriterImpl) ExportToMarkdown(filePath string, result *BatchResult) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create Markdown file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close Markdown file: %v", err)
		}
	}()

	// Write Markdown report
	report := fmt.Sprintf(`# SWE-Bench Batch Processing Report

## Summary

- **Total Tasks**: %d
- **Completed**: %d
- **Failed**: %d
- **Success Rate**: %.2f%%
- **Total Duration**: %s
- **Average Duration**: %s
- **Total Tokens**: %d
- **Total Cost**: $%.4f

## Configuration

- **Model**: %s
- **Workers**: %d
- **Dataset**: %s (%s, %s)

## Error Summary

`,
		result.TotalTasks,
		result.CompletedTasks,
		result.FailedTasks,
		result.SuccessRate,
		result.Duration.String(),
		result.AvgDuration.String(),
		result.TotalTokens,
		result.TotalCost,
		result.Config.Agent.Model.Name,
		result.Config.NumWorkers,
		result.Config.Instances.Type,
		result.Config.Instances.Subset,
		result.Config.Instances.Split,
	)

	if _, err := file.WriteString(report); err != nil {
		return fmt.Errorf("failed to write Markdown report: %w", err)
	}

	// Write error summary table
	if len(result.ErrorSummary) > 0 {
		if _, err := file.WriteString("| Error Type | Count |\n"); err != nil {
			return err
		}
		if _, err := file.WriteString("|------------|-------|\n"); err != nil {
			return err
		}

		for errorType, count := range result.ErrorSummary {
			row := fmt.Sprintf("| %s | %d |\n", errorType, count)
			if _, err := file.WriteString(row); err != nil {
				return err
			}
		}
	} else {
		if _, err := file.WriteString("No errors occurred.\n"); err != nil {
			return err
		}
	}

	return nil
}

// ValidateResults validates that results conform to expected format
func (rw *ResultWriterImpl) ValidateResults(results []WorkerResult) error {
	if len(results) == 0 {
		return fmt.Errorf("no results to validate")
	}

	for i, result := range results {
		if result.InstanceID == "" {
			return fmt.Errorf("result %d missing instance_id", i)
		}

		if result.Status == "" {
			return fmt.Errorf("result %d missing status", i)
		}

		if result.Duration <= 0 {
			return fmt.Errorf("result %d has invalid duration: %v", i, result.Duration)
		}

		// Check if failed results have error information
		if result.Status == StatusFailed && result.Error == "" {
			return fmt.Errorf("result %d has failed status but no error message", i)
		}
	}

	return nil
}

// GetResultsStats returns statistics about the results
func (rw *ResultWriterImpl) GetResultsStats(results []WorkerResult) ResultsStats {
	var stats ResultsStats

	stats.Total = len(results)

	var totalDuration time.Duration
	var totalTokens int
	var totalCost float64
	errorCounts := make(map[string]int)

	for _, result := range results {
		switch result.Status {
		case StatusCompleted:
			stats.Completed++
		case StatusFailed:
			stats.Failed++
			if result.ErrorType != "" {
				errorCounts[result.ErrorType]++
			}
		case StatusTimeout:
			stats.Timeout++
		case StatusCanceled:
			stats.Canceled++
		}

		totalDuration += result.Duration
		totalTokens += result.TokensUsed
		totalCost += result.Cost
	}

	if stats.Total > 0 {
		stats.SuccessRate = float64(stats.Completed) / float64(stats.Total) * 100
		stats.AvgDuration = totalDuration / time.Duration(stats.Total)
	}

	stats.TotalTokens = totalTokens
	stats.TotalCost = totalCost
	stats.ErrorCounts = errorCounts

	return stats
}

// ResultsStats represents statistics about batch processing results
type ResultsStats struct {
	Total       int            `json:"total"`
	Completed   int            `json:"completed"`
	Failed      int            `json:"failed"`
	Timeout     int            `json:"timeout"`
	Canceled    int            `json:"canceled"`
	SuccessRate float64        `json:"success_rate"`
	AvgDuration time.Duration  `json:"avg_duration"`
	TotalTokens int            `json:"total_tokens"`
	TotalCost   float64        `json:"total_cost"`
	ErrorCounts map[string]int `json:"error_counts"`
}
