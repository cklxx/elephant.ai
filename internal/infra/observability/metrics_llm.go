package observability

import (
	"context"
	"fmt"
	"time"

	"alex/internal/shared/modelregistry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (m *MetricsCollector) initLLMMetrics() error {
	var err error
	if m.llmRequests, err = m.meter.Int64Counter("alex.llm.requests.total", metric.WithDescription("Total number of LLM requests"), metric.WithUnit("{request}")); err != nil {
		return fmt.Errorf("failed to create llm_requests counter: %w", err)
	}
	if m.llmTokensInput, err = m.meter.Int64Counter("alex.llm.tokens.input", metric.WithDescription("Total input tokens sent to LLM"), metric.WithUnit("{token}")); err != nil {
		return fmt.Errorf("failed to create llm_tokens_input counter: %w", err)
	}
	if m.llmTokensOutput, err = m.meter.Int64Counter("alex.llm.tokens.output", metric.WithDescription("Total output tokens from LLM"), metric.WithUnit("{token}")); err != nil {
		return fmt.Errorf("failed to create llm_tokens_output counter: %w", err)
	}
	if m.llmLatency, err = m.meter.Float64Histogram("alex.llm.latency", metric.WithDescription("LLM request latency in seconds"), metric.WithUnit("s")); err != nil {
		return fmt.Errorf("failed to create llm_latency histogram: %w", err)
	}
	if m.llmCost, err = m.meter.Float64Counter("alex.cost.total", metric.WithDescription("Total cost of LLM requests"), metric.WithUnit("USD")); err != nil {
		return fmt.Errorf("failed to create llm_cost counter: %w", err)
	}
	if m.toolExecutions, err = m.meter.Int64Counter("alex.tool.executions.total", metric.WithDescription("Total number of tool executions"), metric.WithUnit("{execution}")); err != nil {
		return fmt.Errorf("failed to create tool_executions counter: %w", err)
	}
	if m.toolDuration, err = m.meter.Float64Histogram("alex.tool.duration", metric.WithDescription("Tool execution duration in seconds"), metric.WithUnit("s")); err != nil {
		return fmt.Errorf("failed to create tool_duration histogram: %w", err)
	}
	return nil
}

// RecordLLMRequest records an LLM request.
func (m *MetricsCollector) RecordLLMRequest(ctx context.Context, model string, status string, latency time.Duration, inputTokens, outputTokens int, cost float64) {
	if m == nil {
		return
	}
	if hook := m.testHooks.LLMRequest; hook != nil {
		hook(model, status, latency, inputTokens, outputTokens, cost)
	}
	if m.llmRequests == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("model", model),
		attribute.String("status", status),
	}
	m.llmRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.llmTokensInput.Add(ctx, int64(inputTokens), metric.WithAttributes(attribute.String("model", model)))
	m.llmTokensOutput.Add(ctx, int64(outputTokens), metric.WithAttributes(attribute.String("model", model)))
	m.llmLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attrs...))
	if cost > 0 {
		m.llmCost.Add(ctx, cost, metric.WithAttributes(attribute.String("model", model)))
	}
}

// RecordToolExecution records a tool execution.
func (m *MetricsCollector) RecordToolExecution(ctx context.Context, toolName string, status string, duration time.Duration) {
	if m.toolExecutions == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
		attribute.String("status", status),
	}
	m.toolExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.toolDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attribute.String("tool_name", toolName)))
}

// EstimateCost returns the USD cost for a single LLM request.
// It consults the models.dev registry first and falls back to a $1.5/1M default.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	if info, ok := modelregistry.Lookup(model); ok && info.InputPer1M > 0 {
		return float64(inputTokens)/1e6*info.InputPer1M +
			float64(outputTokens)/1e6*info.OutputPer1M
	}
	return float64(inputTokens+outputTokens) / 1e6 * 1.5
}
