package observability

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	llmports "alex/internal/domain/agent/ports/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservabilityNew_DisabledConfigUsesNoopComponents(t *testing.T) {
	configPath := writeObservabilityConfig(t, `
observability:
  logging:
    level: debug
    format: text
  metrics:
    enabled: false
  tracing:
    enabled: false
`)

	obs, err := New(configPath)
	require.NoError(t, err)
	require.NotNil(t, obs)

	assert.False(t, obs.Config().Metrics.Enabled)
	assert.False(t, obs.Config().Tracing.Enabled)
	assert.Nil(t, obs.Metrics.llmRequests)

	assert.NotPanics(t, func() {
		_, span := obs.Tracer.StartSpan(context.Background(), "disabled")
		span.End()
	})
}

func TestObservabilityNew_InvalidTracingFallsBackToNoopTracer(t *testing.T) {
	configPath := writeObservabilityConfig(t, `
observability:
  metrics:
    enabled: false
  tracing:
    enabled: true
    exporter: bogus
`)

	obs, err := New(configPath)
	require.NoError(t, err)
	require.NotNil(t, obs)

	assert.True(t, obs.Config().Tracing.Enabled)
	assert.NotPanics(t, func() {
		_, span := obs.Tracer.StartSpan(context.Background(), "fallback")
		span.End()
	})
}

func TestObservabilityNew_InvalidMetricsConfigReturnsLoadError(t *testing.T) {
	configPath := writeObservabilityConfig(t, `
observability:
  metrics:
    enabled: true
    prometheus_port: nope
`)

	obs, err := New(configPath)
	require.Error(t, err)
	assert.Nil(t, obs)
	assert.Contains(t, err.Error(), "failed to load observability config")
}

func TestInstrumentedLLMClient_CompleteSuccessRecordsMetrics(t *testing.T) {
	obs := newCoverageTestObservability(t)

	var got struct {
		model        string
		status       string
		latency      time.Duration
		inputTokens  int
		outputTokens int
		cost         float64
	}
	obs.Metrics.SetTestHooks(MetricsTestHooks{
		LLMRequest: func(model, status string, latency time.Duration, inputTokens, outputTokens int, cost float64) {
			got.model = model
			got.status = status
			got.latency = latency
			got.inputTokens = inputTokens
			got.outputTokens = outputTokens
			got.cost = cost
		},
	})

	client := &InstrumentedLLMClient{
		inner: coverageLLMClient{
			model: "gpt-4",
			resp: &ports.CompletionResponse{
				Content:    "done",
				StopReason: "stop",
				Usage: ports.TokenUsage{
					PromptTokens:     1200,
					CompletionTokens: 300,
					TotalTokens:      1500,
				},
			},
		},
		obs: obs,
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hello"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "gpt-4", got.model)
	assert.Equal(t, "success", got.status)
	assert.Equal(t, 1200, got.inputTokens)
	assert.Equal(t, 300, got.outputTokens)
	assert.GreaterOrEqual(t, got.latency, time.Duration(0))
	assert.Positive(t, got.cost)
}

func TestInstrumentedLLMClient_CompleteErrorRecordsMetrics(t *testing.T) {
	obs := newCoverageTestObservability(t)

	var got struct {
		model        string
		status       string
		latency      time.Duration
		inputTokens  int
		outputTokens int
		cost         float64
	}
	obs.Metrics.SetTestHooks(MetricsTestHooks{
		LLMRequest: func(model, status string, latency time.Duration, inputTokens, outputTokens int, cost float64) {
			got.model = model
			got.status = status
			got.latency = latency
			got.inputTokens = inputTokens
			got.outputTokens = outputTokens
			got.cost = cost
		},
	})

	client := &InstrumentedLLMClient{
		inner: coverageLLMClient{
			model: "claude-sonnet",
			err:   errors.New("provider timeout"),
		},
		obs: obs,
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hello"}},
	})
	require.Error(t, err)
	assert.Nil(t, resp)

	assert.Equal(t, "claude-sonnet", got.model)
	assert.Equal(t, "error", got.status)
	assert.GreaterOrEqual(t, got.latency, time.Duration(0))
	assert.Zero(t, got.inputTokens)
	assert.Zero(t, got.outputTokens)
	assert.Zero(t, got.cost)
}

func TestMetricsCollector_HTTPSSETaskHooksPreserveLabelsAndSizes(t *testing.T) {
	collector := &MetricsCollector{}

	var httpCalls []struct {
		method        string
		route         string
		status        int
		duration      time.Duration
		responseBytes int64
	}
	var sseCalls []struct {
		eventType string
		status    string
		sizeBytes int64
	}
	var taskCalls []struct {
		status   string
		duration time.Duration
	}

	collector.SetTestHooks(MetricsTestHooks{
		HTTPServerRequest: func(method, route string, status int, duration time.Duration, responseBytes int64) {
			httpCalls = append(httpCalls, struct {
				method        string
				route         string
				status        int
				duration      time.Duration
				responseBytes int64
			}{method: method, route: route, status: status, duration: duration, responseBytes: responseBytes})
		},
		SSEMessage: func(eventType, status string, sizeBytes int64) {
			sseCalls = append(sseCalls, struct {
				eventType string
				status    string
				sizeBytes int64
			}{eventType: eventType, status: status, sizeBytes: sizeBytes})
		},
		TaskExecution: func(status string, duration time.Duration) {
			taskCalls = append(taskCalls, struct {
				status   string
				duration time.Duration
			}{status: status, duration: duration})
		},
	})

	ctx := context.Background()
	collector.RecordHTTPServerRequest(ctx, "POST", "/api/tasks", 201, 150*time.Millisecond, -1)
	collector.RecordHTTPServerRequest(ctx, "GET", "/health", 200, 20*time.Millisecond, 0)
	collector.RecordSSEMessage(ctx, "task_completed", "delivered", 0)
	collector.RecordSSEMessage(ctx, "task_delta", "", -5)
	collector.RecordTaskExecution(ctx, "cancelled", 3*time.Second)

	require.Len(t, httpCalls, 2)
	assert.Equal(t, "POST", httpCalls[0].method)
	assert.Equal(t, "/api/tasks", httpCalls[0].route)
	assert.Equal(t, 201, httpCalls[0].status)
	assert.EqualValues(t, -1, httpCalls[0].responseBytes)
	assert.Equal(t, "/health", httpCalls[1].route)
	assert.EqualValues(t, 0, httpCalls[1].responseBytes)

	require.Len(t, sseCalls, 2)
	assert.Equal(t, "task_completed", sseCalls[0].eventType)
	assert.Equal(t, "delivered", sseCalls[0].status)
	assert.EqualValues(t, 0, sseCalls[0].sizeBytes)
	assert.Equal(t, "task_delta", sseCalls[1].eventType)
	assert.Equal(t, "", sseCalls[1].status)
	assert.EqualValues(t, -5, sseCalls[1].sizeBytes)

	require.Len(t, taskCalls, 1)
	assert.Equal(t, "cancelled", taskCalls[0].status)
	assert.Equal(t, 3*time.Second, taskCalls[0].duration)
}

func TestSanitizeToolArguments_RedactsNestedCompositeAndHeuristicValues(t *testing.T) {
	args := map[string]any{
		"safe": "value",
		"nested_map_string": map[string]string{
			"Authorization": "Bearer REDACTED_TEST_TOKEN",
			"request_id":    "req-123",
		},
		"nested_list": []any{
			map[string]any{
				"cookie": "session=abc",
			},
			map[string]any{
				"payload": map[string]any{
					"api_hint": "testfixturekeyabcdefghijklmnop",
				},
			},
		},
		"samples": []string{
			"testgithubfixturetokenabcdefghij",
			"short-value",
		},
	}

	sanitized := sanitizeToolArguments(args)
	require.NotNil(t, sanitized)

	assert.Equal(t, "value", sanitized["safe"])

	nestedMap, ok := sanitized["nested_map_string"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "***REDACTED***", nestedMap["Authorization"])
	assert.Equal(t, "req-123", nestedMap["request_id"])

	nestedList, ok := sanitized["nested_list"].([]any)
	require.True(t, ok)

	first, ok := nestedList[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "***REDACTED***", first["cookie"])

	second, ok := nestedList[1].(map[string]any)
	require.True(t, ok)
	payload, ok := second["payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "(hidden)", payload["api_hint"])

	samples, ok := sanitized["samples"].([]any)
	require.True(t, ok)
	assert.Equal(t, "(hidden)", samples[0])
	assert.Equal(t, "short-value", samples[1])
}

func writeObservabilityConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func newCoverageTestObservability(t *testing.T) *Observability {
	t.Helper()

	metrics, err := NewMetricsCollector(MetricsConfig{Enabled: false})
	require.NoError(t, err)
	tracer, err := NewTracerProvider(TracingConfig{Enabled: false})
	require.NoError(t, err)

	return &Observability{
		Logger:  NewLogger(LogConfig{Level: "debug", Format: "json", Output: &bytes.Buffer{}}),
		Metrics: metrics,
		Tracer:  tracer,
	}
}

type coverageLLMClient struct {
	model string
	resp  *ports.CompletionResponse
	err   error
}

func (c coverageLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return c.resp, c.err
}

func (c coverageLLMClient) Model() string {
	return c.model
}

var _ llmports.LLMClient = coverageLLMClient{}
