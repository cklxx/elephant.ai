package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// SLAExecutor wraps a ToolExecutor and records execution metrics via an
// SLACollector. When the collector is nil the wrapper is a transparent
// pass-through.
type SLAExecutor struct {
	delegate  tools.ToolExecutor
	collector *SLACollector
}

// NewSLAExecutor returns a new SLAExecutor. If collector is nil the returned
// executor delegates directly with no instrumentation overhead.
func NewSLAExecutor(delegate tools.ToolExecutor, collector *SLACollector) *SLAExecutor {
	return &SLAExecutor{
		delegate:  delegate,
		collector: collector,
	}
}

// Execute measures the full execution time of the delegate (including retries
// and approval) and records the result via the SLACollector.
func (e *SLAExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if e.collector == nil {
		return e.delegate.Execute(ctx, call)
	}

	start := time.Now()
	result, err := e.delegate.Execute(ctx, call)
	duration := time.Since(start)

	// Determine effective error: either the Go-level error or the in-result
	// error (e.g. from approval rejection or circuit breaker).
	recordErr := err
	if recordErr == nil && result != nil && result.Error != nil {
		recordErr = result.Error
	}

	toolName := call.Name
	if toolName == "" {
		toolName = e.delegate.Metadata().Name
	}

	e.collector.RecordExecutionWithCost(toolName, duration, recordErr, extractCostUSD(result))
	return result, err
}

// Definition delegates to the wrapped executor.
func (e *SLAExecutor) Definition() ports.ToolDefinition {
	return e.delegate.Definition()
}

// Metadata delegates to the wrapped executor.
func (e *SLAExecutor) Metadata() ports.ToolMetadata {
	return e.delegate.Metadata()
}

// Delegate returns the wrapped executor. This is used by unwrapTool in the
// registry to peel off layers when re-wrapping a tool.
func (e *SLAExecutor) Delegate() tools.ToolExecutor {
	return e.delegate
}

func extractCostUSD(result *ports.ToolResult) float64 {
	if result == nil || result.Metadata == nil {
		return 0
	}
	raw, ok := result.Metadata["cost_usd"]
	if !ok || raw == nil {
		return 0
	}
	switch typed := raw.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint64:
		return float64(typed)
	case uint32:
		return float64(typed)
	case json.Number:
		v, err := typed.Float64()
		if err != nil {
			return 0
		}
		return v
	case string:
		v, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0
		}
		return v
	default:
		return 0
	}
}

// Verify interface compliance at compile time.
var _ tools.ToolExecutor = (*SLAExecutor)(nil)
